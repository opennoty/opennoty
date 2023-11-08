package peer_transport

import (
	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/sec"
	"github.com/opennoty/opennoty/server/api/peer_proto"
	"net"
	"sync"
	"sync/atomic"
)

type Connection struct {
	incoming   bool
	secureConn sec.SecureConn
	reader     Reader
	closed     int32

	lock sync.Mutex
}

func (c *Connection) Read() (*peer_proto.Payload, error) {
	return c.reader.Read()
}

func (c *Connection) Write(payload *peer_proto.Payload) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if atomic.LoadInt32(&c.closed) != 0 {
		return net.ErrClosed
	}
	return WritePayload(c.secureConn, payload)
}

func (c *Connection) IsIncoming() bool {
	return c.incoming
}

func (c *Connection) Close() error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if atomic.SwapInt32(&c.closed, 1) != 0 {
		return nil
	}
	return c.secureConn.Close()
}

func (c *Connection) RemoteAddr() net.Addr {
	return c.secureConn.RemoteAddr()
}

// LocalPeer returns our peer ID
func (c *Connection) LocalPeer() peer.ID {
	return c.secureConn.LocalPeer()
}

// RemotePeer returns the peer ID of the remote peer.
func (c *Connection) RemotePeer() peer.ID {
	return c.secureConn.RemotePeer()
}

// RemotePublicKey returns the public key of the remote peer.
func (c *Connection) RemotePublicKey() ic.PubKey {
	return c.secureConn.RemotePublicKey()
}
