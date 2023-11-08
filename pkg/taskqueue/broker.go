package taskqueue

import (
	"context"
	"github.com/opennoty/opennoty/pkg/health"
)

type Message interface {
	Body() []byte
	Parse(jsonObj interface{}) error
}

type Client interface {
	health.Provider
	Broker(routingKey string) (Broker, error)
	Shutdown() error
}

type Broker interface {
	Publish(body interface{}) error
	Consume(ctx context.Context) (chan Message, error)
	Ack(msg Message) error
	Nack(msg Message, requeue bool) error
}
