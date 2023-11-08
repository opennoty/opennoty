package peer_service

import (
	"github.com/gofiber/fiber/v2/log"
	"github.com/opennoty/opennoty/server/peer_transport"
	"math/rand"
	"time"
)

func (s *peerService) protocolHandler(p *Peer, connection *peer_transport.Connection) {
	s.protocolReader(p, connection)
	p.Close()

	s.reconnectIfNeeded(p)
}

func (s *peerService) reconnectIfNeeded(p *Peer) {
	if !p.enabled {
		return
	}

	log.Infof("Peer[%s] try reconnect after few seconds", p.peerId.String())
	time.Sleep(time.Duration(1000+rand.Intn(2000)) * time.Millisecond)

	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.enabled || p.state != PeerNotConnected {
		return
	}

	p.state = PeerConnecting
	go s.connectTo(p)
}

func (s *peerService) protocolReader(p *Peer, connection *peer_transport.Connection) {
	for {
		payload, err := connection.Read()
		if err != nil {
			log.Warnf("Peer[%s]: read failed: %v", connection.RemotePeer(), err)
			break
		}

		handler, ok := s.payloadHandlers[payload.Type]
		if ok {
			handler(p, payload)
		}
	}
}
