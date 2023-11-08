package mongo_util

import (
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

type MongoUpdateStatement struct {
	Set         bsoncore.Document `bson:"$set,omitempty"`
	SetOnInsert bsoncore.Document `bson:"$setOnInsert,omitempty"`
}
