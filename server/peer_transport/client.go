package peer_transport

import (
	"context"
	"github.com/gofiber/fiber/v2/log"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"net"
	"time"
)

func Connect(ctx context.Context, noiseTransport *noise.Transport, address string, peerId peer.ID) (*Connection, error) {
	insecureConn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return nil, err
	}

	handshakeCtx, _ := context.WithTimeout(ctx, time.Second*3)
	secureConn, err := noiseTransport.SecureOutbound(handshakeCtx, insecureConn, peerId)
	if err != nil {
		log.Warnf("accept handshake failed: %v", err)
		return nil, err
	}

	c := &Connection{
		secureConn: secureConn,
		reader: Reader{
			conn: secureConn,
		},
	}

	return c, nil
}
