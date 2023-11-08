package pubsub_service

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
)

type TopicKey string
type PeerId string
type SubscribeKey string
type SubscribeHandler func(tenantId string, topic string, data []byte, subscribeKey SubscribeKey) error

type PubSubService interface {
	Start(appCtx context.Context, mongoDatabase *mongo.Database) error
	Stop()
	Publish(tenantId string, topic string, data []byte) error
	Subscribe(tenantId string, topic string, handler SubscribeHandler) SubscribeKey
	Unsubscribe(subscribeKey SubscribeKey)
	Unsubscribes(subscribeKeys []SubscribeKey)
}
