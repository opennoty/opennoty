package peer_service

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type PeerDocument struct {
	Id          primitive.ObjectID `bson:"_id,omitempty"`
	PeerId      string             `bson:"peerId"`
	HeartbeatAt time.Time          `bson:"heartbeatAt"`
	Address     string             `bson:"fullAddress"`
	Port        int                `bson:"port"`
}

func (d *PeerDocument) GetId() string {
	return d.Id.String()
}

func (d *PeerDocument) ToBson() (bson.M, error) {
	data, err := bson.Marshal(d)
	if err != nil {
		return nil, err
	}

	var result bson.M
	err = bson.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (d *PeerDocument) ToFilter() bson.M {
	return bson.M{
		"peerId": d.PeerId,
	}
}

func (d *PeerDocument) ToUpdate(newId primitive.ObjectID) (bson.M, error) {
	data, err := bson.Marshal(d)
	if err != nil {
		return nil, err
	}

	var doc bson.M
	err = bson.Unmarshal(data, &doc)
	if err != nil {
		return nil, err
	}

	delete(doc, "_id")

	return bson.M{
		"$set": doc,
		"$setOnInsert": bson.M{
			"_id": newId,
		},
	}, nil
}

var peerIndexes = []mongo.IndexModel{
	{
		Keys: bson.M{
			"peerId": 1,
		},
		Options: options.Index().SetUnique(true),
	},
	{
		Keys: bson.M{
			"heartbeatAt": 1,
		},
		Options: options.Index().SetExpireAfterSeconds(60),
	},
}
