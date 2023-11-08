package notification_service

import (
	"github.com/opennoty/opennoty/api/internal_bson"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type NotificationDocument struct {
	Id        primitive.ObjectID `bson:"_id,omitempty"`
	TenantId  string             `bson:"tenantId"`
	AccountId string             `bson:"accountId"`
	Subject   string             `bson:"subject"`
	Step      internal_bson.Step `bson:"step"`

	ReadMarked bool `bson:"readMarked"`
	Deleted    bool `bson:"deleted"`
}

func (d *NotificationDocument) GetId() string {
	return d.Id.String()
}

func (d *NotificationDocument) ToUpdate(newId primitive.ObjectID) (bson.M, error) {
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

var notificationIndexes = []mongo.IndexModel{
	{
		Keys: bson.D{
			{"accountId", 1},
			{"_id", -1},
		},
	},
}
