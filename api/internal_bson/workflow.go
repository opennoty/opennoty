package internal_bson

import (
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

type Step struct {
	Digest     bool                `bson:"digest"`
	Events     []bsoncore.Document `bson:"events"`
	TotalCount int                 `bson:"totalCount"`
}
