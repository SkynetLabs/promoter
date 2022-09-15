package database

import (
	"context"
	"gitlab.com/NebulousLabs/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type (
	// User identifies a portal user by their sub.
	User struct {
		Sub string `bson:"sub"`
	}

	// Subscription describes a single subscription period.
	Subscription struct {
		ID    primitive.ObjectID `bson:"_id"`
		Sub   string             `bson:"sub"`
		Tier  int                `bson:"tier"`
		From  time.Time          `bson:"from"`
		To    time.Time          `bson:"to"`
		Price float64            `bson:"price"`
	}

	// Txn represents a transfer of cryptocurrency with a txn ID and an amount
	// of credits that the txn's sum amounts to. The conversion is done by the
	// appropriate payment processor.
	Txn struct {
		ID     string  `bson:"_id"`
		Sub    string  `bson:"sub"`
		Amount float64 `bson:"amount"` // credits
	}
)

// CreditUser adds the given amount to the user's credit balance and marks the
// txnID as processed. If the txn is already processed, this is a no-op.
// This method assumes that it's called from within a DB transaction, so when
// it fails with an error all changes in the DB are automatically rolled back.
func (db *DB) CreditUser(ctx context.Context, sub string, amount float64, txnID string) error {
	// Make sure the user exists.
	_, err := db.NewUser(ctx, sub)
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return errors.AddContext(err, "failed to create user")
	}
	// Register txn.
	_, err = db.NewTxn(ctx, txnID, sub, amount)
	if mongo.IsDuplicateKeyError(err) {
		// This txn has already been processed, nothing to do.
		return nil
	}
	if err != nil {
		return errors.AddContext(err, "failed to register txn")
	}
	return nil
}

// NewUser creates a new user with the given sub.
func (db *DB) NewUser(ctx context.Context, sub string) (*User, error) {
	u := &User{Sub: sub}
	_, err := db.staticDB.Collection(collUsers).InsertOne(ctx, u)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// NewTxn creates a new txn in the DB.
func (db *DB) NewTxn(ctx context.Context, id string, sub string, amount float64) (*Txn, error) {
	txn := &Txn{
		ID:     id,
		Sub:    sub,
		Amount: amount,
	}
	_, err := db.staticDB.Collection(collTnxs).InsertOne(ctx, txn)
	if err != nil {
		return nil, err
	}
	return txn, nil
}

// UserBalance returns the current balance of credits for the given sub.
func (db *DB) UserBalance(ctx context.Context, sub string) (float64, error) {
	credit, err := db.userCredit(ctx, sub)
	if err != nil {
		return 0, errors.AddContext(err, "failed to calculate the total amount of credit")
	}
	spent, err := db.userSpent(ctx, sub)
	if err != nil {
		return 0, errors.AddContext(err, "failed to calculate the total amount spent")
	}
	return credit - spent, nil
}

// userCredit returns the total amount of credits ever credited to this sub.
func (db *DB) userCredit(ctx context.Context, sub string) (float64, error) {
	match := bson.D{{"$match", bson.D{{"sub", sub}}}}
	group := bson.D{{
		"$group", bson.D{
			{"_id", bson.D{{"sub", "$sub"}}},
			{"credit", bson.D{{"$sum", "$amount"}}},
		},
	}}
	c, err := db.staticDB.Collection(collTnxs).Aggregate(ctx, mongo.Pipeline{match, group})
	if err != nil {
		return 0, err
	}
	txns := struct {
		Credit float64 `bson:"credit"`
	}{}
	// We only parse if we have a result. If we don't have a result, that means
	// that there are no txns and the total credit is zero.
	if c.Next(ctx) {
		err = c.Decode(&txns)
		if err != nil {
			return 0, err
		}
	}
	return txns.Credit, nil
}

// userSpent returns the total amount of credits ever spent by this sub.
func (db *DB) userSpent(ctx context.Context, sub string) (float64, error) {
	match := bson.D{{"$match", bson.D{{"sub", sub}}}}
	group := bson.D{{
		"$group", bson.D{
			{"_id", bson.D{{"sub", "$sub"}}},
			{"spent", bson.D{{"$sum", "$price"}}},
		},
	}}
	c, err := db.staticDB.Collection(collTnxs).Aggregate(ctx, mongo.Pipeline{match, group})
	if err != nil {
		return 0, err
	}
	subs := struct {
		Spent float64 `bson:"spent"`
	}{}
	// We only parse if we have a result. If we don't have a result, that means
	// that there are no txns and the total spent amount is zero.
	if c.Next(ctx) {
		err = c.Decode(&subs)
		if err != nil {
			return 0, err
		}
	}
	return subs.Spent, nil
}
