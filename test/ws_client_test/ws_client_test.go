package ws_client_test

import (
	"fmt"
	"github.com/opennoty/opennoty/pkg/websocket_client"
	"github.com/opennoty/opennoty/test"
	"github.com/opennoty/opennoty/test/testhelper"
	"github.com/stretchr/testify/assert"
	"log"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	testhelper.SetTimeout(time.Second * 10)
	testhelper.TestMain(m)
}

func TestWebSocketSubscribe(t *testing.T) {
	testEnv := testhelper.GetTestEnv()

	servers := test.CreateServers(t, testEnv.MongoServer, 2)
	if servers == nil {
		return
	}
	defer servers.Shutdown()

	c0, err := websocket_client.Dial(servers.GetServerUrl(0), nil)
	if err != nil {
		t.Error(err)
		return
	}
	defer c0.Close()

	c1, err := websocket_client.Dial(servers.GetServerUrl(0), nil)
	if err != nil {
		t.Error(err)
		return
	}
	defer c1.Close()

	expectedMessageList := make([]string, 10000)
	for i := range expectedMessageList {
		expectedMessageList[i] = string(fmt.Sprintf("TEST MESSAGE %d: "+strings.Repeat("a", 256), i))
	}
	var c0ReceivedList []string
	var c1ReceivedList []string

	var startedAt time.Time
	var endedAt time.Time

	c0.Subscribe(testEnv.Ctx, "test-topic", func(topic string, data []byte, subscribeKey string) {
		c0ReceivedList = append(c0ReceivedList, string(data))
		if len(c0ReceivedList) == len(expectedMessageList) {
			endedAt = time.Now()
		}
	})
	c1.Subscribe(testEnv.Ctx, "test-topic", func(topic string, data []byte, subscribeKey string) {
		c1ReceivedList = append(c1ReceivedList, string(data))
	})

	startedAt = time.Now()
	for i, s := range expectedMessageList {
		if i < len(expectedMessageList)/2 {
			servers[0].PubSubService().Publish("", "test-topic", []byte(s))
		} else {
			servers[1].PubSubService().Publish("", "test-topic", []byte(s))
		}
	}

	time.Sleep(time.Second)
	milli := endedAt.Sub(startedAt).Milliseconds()

	qps := float32(len(expectedMessageList)) / (float32(milli) / 1000)
	log.Println("qps: ", qps)

	assert.Equal(t, len(expectedMessageList), len(c0ReceivedList))
	assert.Equal(t, len(expectedMessageList), len(c1ReceivedList))
	assert.Equal(t, expectedMessageList, c0ReceivedList)
	assert.Equal(t, expectedMessageList, c1ReceivedList)
}
