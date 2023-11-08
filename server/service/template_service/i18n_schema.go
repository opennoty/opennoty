package template_service

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type I18nDocument struct {
	Id      primitive.ObjectID `bson:"_id,omitempty"`
	Locale  string             `bson:"locale"`
	Name    string             `bson:"name"`
	Message string             `bson:"message"`
}

func (d *I18nDocument) GetId() string {
	return d.Id.String()
}

func (d *I18nDocument) ToBson() (bson.M, error) {
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

var i18nIndexes = []mongo.IndexModel{
	{
		Keys: bson.D{
			{"locale", 1},
			{"name", 1},
		},
		Options: options.Index().SetUnique(true),
	},
}
