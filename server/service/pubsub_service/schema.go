package pubsub_service

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type SubscriberDocument struct {
	Id          primitive.ObjectID `bson:"_id,omitempty"`
	TenantId    string             `bson:"tenantId"`
	Topic       string             `bson:"topic"`
	PeerId      string             `bson:"peerId"`
	HeartbeatAt time.Time          `bson:"heartbeatAt"`
}

func (d *SubscriberDocument) GetId() string {
	return d.Id.String()
}

func (d *SubscriberDocument) ToBson() (bson.M, error) {
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

func (d *SubscriberDocument) TopicKey() TopicKey {
	return makeTopicKey(d.TenantId, d.Topic)
}

var subscriberIndexes = []mongo.IndexModel{
	{
		Keys: bson.M{
			"peerId": 1,
		},
		Options: options.Index(),
	},
	{
		Keys: bson.M{
			"heartbeatAt": 1,
		},
		Options: options.Index().SetExpireAfterSeconds(3600),
	},
	{
		Keys: bson.D{
			{"tenantId", 1},
			{"topic", 1},
		},
		Options: options.Index(),
	},
}
