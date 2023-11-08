package peer_service

import (
	"context"
	"github.com/opennoty/opennoty/server/api/peer_proto"
	"go.mongodb.org/mongo-driver/mongo"
)

type PeerService interface {
	Start(appCtx context.Context, listenAddress string, peerNotifyAddress string, mongoDatabase *mongo.Database) error
	Stop()
	RegisterPayloadHandler(payloadType peer_proto.PayloadType, handler PayloadHandler)
	GetPeerId() string
	GetPeers() []*Peer
	NotifyTopic(peerId string, tenantId string, topicName string, data []byte) error
}
