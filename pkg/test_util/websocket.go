package test_util

import (
	"golang.org/x/net/websocket"
	"net/http/httptest"
	"strings"
	"testing"
)

func MakePair(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	serverConnCh := make(chan *websocket.Conn, 1)
	shutdownCh := make(chan bool, 1)

	var webSocketHandler websocket.Handler = func(conn *websocket.Conn) {
		serverConnCh <- conn
		<-shutdownCh
	}

	s := httptest.NewServer(webSocketHandler)

	u := "ws" + strings.TrimPrefix(s.URL, "http")

	// Connect to the server
	clientConn, err := websocket.Dial(u, "", s.URL)
	if err != nil {
		t.Fatalf("%v", err)
	}

	return <-serverConnCh, clientConn, func() {
		close(shutdownCh)
		s.Close()
	}
}
