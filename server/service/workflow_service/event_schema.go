package workflow_service

import (
	"github.com/opennoty/opennoty/api/api_model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"time"
)

type FlowState int

const (
	FlowStateWaiting   FlowState = 0
	FlowStateProcessed FlowState = 1
	FlowStateError     FlowState = 2
)

type EventType string

const (
	EventTypeTrigger EventType = "trigger"
	EventTypeDigest  EventType = "digest"
)

type DigestData struct {
	EventIds []primitive.ObjectID `bson:"eventIds"`
}

type EventDocument struct {
	Id               primitive.ObjectID   `bson:"_id,omitempty"`
	Tenant           api_model.Tenant     `bson:"tenant"`
	Subscriber       api_model.Subscriber `bson:"subscriber"`
	WorkflowName     string               `bson:"workflowName"`
	WorkflowRevision int64                `bson:"workflowRevision"`
	EventType        EventType            `bson:"eventType"`
	NextAfterAt      time.Time            `bson:"nextAfterAt"`
	Finished         bool                 `bson:"finished"`

	// Data and FlowState is available when EventType == EventTypeTrigger
	Data      bsoncore.Document `bson:"data,omitempty"`
	FlowState []FlowState       `bson:"flowState"`
	FlowError []*string         `bson:"flowError"`

	DigestData *DigestData `bson:"digestData,omitempty"`
}

var (
	metaWhiteList = []string{
		"tenant", "subscriber", "workflowName", "workflowRevision",
	}
)

func (d *EventDocument) GetId() string {
	return d.Id.String()
}

func (d *EventDocument) ToBson() (bson.M, error) {
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

func (d *EventDocument) ToBsonForMeta() (bson.M, error) {
	var result = bson.M{}
	orig, err := d.ToBson()
	if err != nil {
		return nil, err
	}
	for _, key := range metaWhiteList {
		result[key] = orig[key]
	}
	return result, nil
}

func (d *EventDocument) DataFrom(data interface{}) error {
	raw, err := bson.Marshal(data)
	if err != nil {
		return err
	}
	d.Data = raw
	return nil
}

var eventIndexes = []mongo.IndexModel{}
