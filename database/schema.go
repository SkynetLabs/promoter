package database

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// schema returns a mapping between a collection name and the indexes that
// must exist for that collection.
//
// We return a map literal instead of using a global variable because the global
// variable causes data races when multiple tests are creating their own
// databases and are iterating over the schema at the same time.
func schema() map[string][]mongo.IndexModel {
	return map[string][]mongo.IndexModel{
		collSubscriptions: {
			{
				Keys:    bson.D{{"sub", 1}},
				Options: options.Index().SetName("sub"),
			},
			{
				Keys:    bson.D{{"from", 1}},
				Options: options.Index().SetName("from"),
			},
			{
				Keys:    bson.D{{"to", 1}},
				Options: options.Index().SetName("to"),
			},
		},
		collTnxs: {
			{
				Keys:    bson.D{{"price", 1}},
				Options: options.Index().SetName("price"),
			},
		},
	}
}
