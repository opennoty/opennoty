package mongo_util

import "go.mongodb.org/mongo-driver/bson/primitive"

type DocumentKey struct {
	Id primitive.ObjectID `bson:"_id"`
}

type WatchChangeEvent[D any] struct {
	OperationType string      `bson:"operationType"`
	FullDocument  D           `bson:"fullDocument,omitempty"`
	DocumentKey   DocumentKey `bson:"documentKey"`
}
