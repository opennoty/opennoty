package test

import (
	"fmt"
	mim "github.com/ONSdigital/dp-mongodb-in-memory"
	"github.com/opennoty/opennoty/server"
	"github.com/opennoty/opennoty/server/service/peer_service"
	"net/url"
	"sync"
	"testing"
	"time"
)

type ServerList []*server.Server

func CreateServers(t *testing.T, mongoServer *mim.Server, count int) ServerList {
	var list ServerList

	mongoUri, err := url.Parse(mongoServer.URI())
	if err != nil {
		t.Error(err)
		return nil
	}
	mongoUri = mongoUri.JoinPath("test")

	for i := 0; i < count; i++ {
		app, err := server.NewServer(&server.Option{
			MongoUri:    mongoUri.String(),
			PublicPort:  -1,
			PrivatePort: -1,
		})
		if err != nil {
			t.Error(err)
			return nil
		}
		list = append(list, app)
	}

	for _, s := range list {
		if err := s.Start(); err != nil {
			list.Shutdown()
			t.Error(err)
			return nil
		}
	}

	var wg sync.WaitGroup
	for _, s := range list {
		wg.Add(1)
		go func(s *server.Server) {
			if err := s.FiberServe(); err != nil {
				t.Error(err)
				wg.Done()
			}
		}(s)
		go func(s *server.Server) {
			for s.Option().PublicPort == 0 {
				time.Sleep(time.Millisecond * 100)
			}
			wg.Done()
		}(s)
	}

	wg.Wait()

	for {
		peers := list[0].PeerService().GetPeers()
		if len(peers) >= (count-1) && peers[0].GetState() == peer_service.PeerConnected {
			break
		}
		time.Sleep(time.Millisecond * 100)
	}

	return list
}

func (ss ServerList) GetServerUrl(index int) string {
	s := ss[index]
	return fmt.Sprintf("ws://localhost:%d%s", s.Option().PublicPort, s.Option().ContextPath)
}

func (ss ServerList) Shutdown() {
	for _, s := range ss {
		s.Stop()
	}
}
