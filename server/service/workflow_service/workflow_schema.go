package workflow_service

import (
	"github.com/opennoty/opennoty/api/api_model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type WorkflowDocument struct {
	Id       primitive.ObjectID            `bson:"_id,omitempty" json:"_id,omitempty"`
	Name     string                        `bson:"name" json:"name"`
	Revision int64                         `bson:"revision" json:"revision"`
	Flow     []*api_model.WorkflowFlowItem `bson:"flow" json:"flow"`
}

func (d *WorkflowDocument) GetId() string {
	return d.Id.String()
}

func (d *WorkflowDocument) ToBson() (bson.M, error) {
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

var workflowIndexes = []mongo.IndexModel{
	{
		Keys: bson.D{
			{"name", 1},
			{"revision", -1},
		},
		Options: options.Index().SetUnique(true),
	},
}
