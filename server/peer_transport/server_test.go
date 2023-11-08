package peer_transport

import (
	"context"
	"crypto/rand"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/opennoty/opennoty/server/api/peer_proto"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestBench(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Error(err)
		return
	}
	serverPeerId, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		t.Error(err)
		return
	}
	noiseTransport, err := noise.New(noise.ID, privKey, nil)
	if err != nil {
		t.Error(err)
		return
	}

	server := NewServer(ctx, noiseTransport)
	listener, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Error(err)
		return
	}
	defer listener.Close()

	var count int32

	go func() {
		for ctx.Err() == nil {
			insecure, err := listener.Accept()
			opErr, _ := err.(*net.OpError)
			if opErr != nil {
				err = opErr.Unwrap()
			}
			if err == net.ErrClosed {
				break
			} else if err != nil {
				t.Error(err)
				break
			}

			go func() {
				conn, err := server.Handshake(insecure)
				if err != nil {
					t.Error(err)
					return
				}

				for ctx.Err() == nil {
					payload, err := conn.Read()
					if err == io.EOF {
						break
					} else if err != nil {
						t.Error(err)
					}
					_ = payload
					atomic.AddInt32(&count, 1)
				}
			}()
		}
	}()

	for _ = range [32]int{} {
		go func() {
			var payload peer_proto.Payload
			payload.TopicName = "HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456"
			conn, err := Connect(context.Background(), noiseTransport, listener.Addr().String(), serverPeerId)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			for ctx.Err() == nil {
				err := conn.Write(&payload)
				if err != nil {
					t.Error(err)
				}
			}
		}()
	}

	testSeconds := 2

	timer := time.NewTimer(time.Second * time.Duration(testSeconds))
	<-timer.C
	qps := float32(atomic.LoadInt32(&count)) / float32(testSeconds)
	cancel()

	assert.Greater(t, qps, float32(32))

	log.Println("qps: ", int(qps))
}
