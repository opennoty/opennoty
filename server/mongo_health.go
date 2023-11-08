package server

import (
	"context"
	"github.com/opennoty/opennoty/pkg/health"
	"go.mongodb.org/mongo-driver/mongo"
)

type mongoHealthProvider struct {
	mongoDatabase *mongo.Database
}

func (p *mongoHealthProvider) Check() health.Health {
	resp := health.Health{
		Status: health.StatusUp,
	}
	if err := p.mongoDatabase.Client().Ping(context.Background(), nil); err != nil {
		resp.Status = health.StatusDown
		resp.Reason = err.Error()
	}
	return resp
}
