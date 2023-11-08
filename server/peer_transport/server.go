package peer_transport

import (
	"context"
	"errors"
	"github.com/gofiber/fiber/v2/log"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"net"
	"time"
)

type Server struct {
	ctx   context.Context
	noise *noise.Transport
}

func NewServer(ctx context.Context, noise *noise.Transport) *Server {
	return &Server{
		ctx:   ctx,
		noise: noise,
	}
}

func (s *Server) Handshake(insecureConn net.Conn) (*Connection, error) {
	ctx, _ := context.WithTimeout(s.ctx, time.Second*3)

	secureConn, err := s.noise.SecureInbound(ctx, insecureConn, "")
	if err != nil {
		log.Warnf("accept handshake failed: %v", err)
		return nil, err
	}

	c := &Connection{
		incoming:   true,
		secureConn: secureConn,
		reader: Reader{
			conn: secureConn,
		},
	}
	errors.Join()

	return c, nil
}
