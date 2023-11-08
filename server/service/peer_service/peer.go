package peer_service

import (
	"fmt"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/opennoty/opennoty/server/api/peer_proto"
	"github.com/opennoty/opennoty/server/peer_transport"
	"sync"
)

type PeerState int32

const (
	PeerNotConnected PeerState = iota
	PeerConnecting   PeerState = iota
	PeerConnected    PeerState = iota
)

type Peer struct {
	peerDocument PeerDocument
	lock         sync.Mutex
	state        PeerState
	fullAddress  string
	peerId       peer.ID

	enabled    bool
	connection *peer_transport.Connection
}

func (p *Peer) PeerId() string {
	return p.peerId.String()
}

func (p *Peer) Connected(connection *peer_transport.Connection) {
	p.state = PeerConnected
	p.connection = connection
}

func (p *Peer) Close() error {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.state = PeerNotConnected
	if p.connection == nil {
		return nil
	}
	err := p.connection.Close()
	p.connection = nil
	return err
}

func (p *Peer) GetState() PeerState {
	return p.state
}

func (p *Peer) GetConnection() *peer_transport.Connection {
	return p.connection
}

func (p *Peer) Write(payload *peer_proto.Payload) error {
	var connection *peer_transport.Connection

	p.lock.Lock()
	connection = p.connection
	state := p.state
	p.lock.Unlock()
	if connection == nil || state != PeerConnected {
		return fmt.Errorf("not connected")
	}

	return connection.Write(payload)
}

func (s PeerState) String() string {
	switch s {
	case PeerNotConnected:
		return "PeerNotConnected"
	case PeerConnecting:
		return "PeerConnecting"
	case PeerConnected:
		return "PeerConnected"
	}
	return ""
}
