package mongo_leader

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type LeaderDocument struct {
	Id          primitive.ObjectID `bson:"_id,omitempty"`
	Key         string             `bson:"key"`
	PeerId      string             `bson:"peerId"`
	HeartbeatAt time.Time          `bson:"heartbeatAt"`
}

var leaderIndexes = []mongo.IndexModel{
	{
		Keys: bson.M{
			"key": 1,
		},
		Options: options.Index().SetUnique(true),
	},
	{
		Keys: bson.M{
			"heartbeatAt": 1,
		},
		Options: options.Index().SetExpireAfterSeconds(10),
	},
}
