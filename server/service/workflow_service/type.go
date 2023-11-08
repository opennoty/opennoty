package workflow_service

import (
	"context"
	"errors"
	"github.com/opennoty/opennoty/api/api_model"
	"github.com/opennoty/opennoty/pkg/mongo_leader"
	"github.com/opennoty/opennoty/pkg/taskqueue"
	"go.mongodb.org/mongo-driver/mongo"
)

type TriggerParams struct {
	Name       string               `json:"name"`
	Tenant     api_model.Tenant     `json:"tenant"`
	Subscriber api_model.Subscriber `json:"subscriber"`
	Event      map[string]any       `json:"event"`
}

type WorkflowService interface {
	Start(appCtx context.Context, mongoDatabase *mongo.Database, brokerClient taskqueue.Client, leaderElection *mongo_leader.LeaderElection) error
	Trigger(params *TriggerParams) (eventId string, err error)
	CreateWorkflow(doc *WorkflowDocument) (*WorkflowDocument, error)
	GetWorkflow(name string) (*WorkflowDocument, error)
}

var ErrNotFound = errors.New("not found")
