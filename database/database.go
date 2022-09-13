package database

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

type (
	// Health contains health information about the promoter. Namely, the
	// database. If everything is ok all fields are 'nil'.
	// Otherwise, the corresponding fields will contain an error.
	Health struct {
		Database error
	}

	// DB is a wrapper around a database client.
	DB struct {
		staticDB           *mongo.Database
		staticLogger       *logrus.Entry
		staticServerDomain string

		staticCtx          context.Context
		staticBGCtx        context.Context
		staticThreadCancel context.CancelFunc
		staticWG           sync.WaitGroup
	}
)

// New creates a new promoter from the given db credentials.
func New(ctx context.Context, log *logrus.Entry, uri, username, password, domain, dbName string) (*DB, error) {
	dbClient, err := connect(ctx, uri, username, password)
	if err != nil {
		return nil, err
	}
	return newDB(ctx, log, dbClient, domain, dbName), nil
}

// connect creates a new database object that is connected to a mongodb.
func connect(ctx context.Context, uri, username, password string) (*mongo.Client, error) {
	// Connect to database.
	creds := options.Credential{
		Username: username,
		Password: password,
	}
	opts := options.Client().
		ApplyURI(uri).
		SetAuth(creds).
		SetReadConcern(readconcern.Majority()).
		SetReadPreference(readpref.Nearest()).
		SetWriteConcern(writeconcern.New(writeconcern.WMajority()))

	c, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// newDB creates a new promoter object from a given db client.
func newDB(ctx context.Context, log *logrus.Entry, client *mongo.Client, domain, dbName string) *DB {
	// Create a new context for background threads.
	bgCtx, cancel := context.WithCancel(ctx)

	return &DB{
		staticDB:           client.Database(dbName),
		staticLogger:       log,
		staticServerDomain: domain,

		staticCtx:          ctx,
		staticBGCtx:        bgCtx,
		staticThreadCancel: cancel,
	}
}

// Close gracefully shuts down the DB.
func (db *DB) Close() error {
	return db.staticDB.Client().Disconnect(context.Background())
}

// Health returns some health information about the promoter.
func (db *DB) Health() Health {
	return Health{
		Database: db.staticDB.Client().Ping(db.staticCtx, nil),
	}
}
