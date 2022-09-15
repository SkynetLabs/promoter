package database

import (
	"context"
	"gitlab.com/NebulousLabs/errors"
	"sync"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

const (
	// DBName is the name of the database to use for Promoter.
	DBName = "promoter"

	// collSubscriptions defines the name of the collection which will hold
	// information about users' subscriptions.
	collSubscriptions = "subscriptions"

	// collTnxs defines the name of the collection which will hold
	// information about txns. The most important bit here is to keep a solid
	// record of processed txns, so we never double-process a txn and all calls
	// to promoter from the payment processors can be idempotent.
	collTnxs = "txns"

	// collUsers defines the name of the collection which will hold
	// information about users, such as their sub and current balance.
	collUsers = "users"
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
	return newDB(ctx, log, dbClient, domain, dbName)
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
	return mongo.Connect(ctx, opts)
}

// newDB creates a new promoter object from a given db client.
func newDB(ctx context.Context, log *logrus.Entry, client *mongo.Client, domain, dbName string) (*DB, error) {
	db := client.Database(dbName)
	err := ensureDBSchema(ctx, db, log)
	if err != nil {
		return nil, err
	}
	// Create a new context for background threads.
	bgCtx, cancel := context.WithCancel(ctx)
	return &DB{
		staticDB:           db,
		staticLogger:       log,
		staticServerDomain: domain,

		staticCtx:          ctx,
		staticBGCtx:        bgCtx,
		staticThreadCancel: cancel,
	}, nil
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

// NewSession starts a new Mongo session.
func (db *DB) NewSession() (mongo.Session, error) {
	return db.staticDB.Client().StartSession()
}

// ensureDBSchema checks that we have all collections and indexes we need and
// creates them if needed.
// See https://docs.mongodb.com/manual/indexes/
// See https://docs.mongodb.com/manual/core/index-unique/
func ensureDBSchema(ctx context.Context, db *mongo.Database, log *logrus.Entry) error {
	for collName, models := range schema() {
		coll, err := ensureCollection(ctx, db, collName)
		if err != nil {
			return err
		}
		iv := coll.Indexes()
		names, err := iv.CreateMany(ctx, models)
		if err != nil {
			return errors.AddContext(err, "failed to create indexes")
		}
		log.Debugf("Ensured index exists: %v", names)
	}
	return nil
}

// ensureCollection gets the given collection from the
// database and creates it if it doesn't exist.
func ensureCollection(ctx context.Context, db *mongo.Database, collName string) (*mongo.Collection, error) {
	coll := db.Collection(collName)
	if coll == nil {
		err := db.CreateCollection(ctx, collName)
		if err != nil {
			return nil, err
		}
		coll = db.Collection(collName)
		if coll == nil {
			return nil, errors.New("failed to create collection " + collName)
		}
	}
	return coll, nil
}
