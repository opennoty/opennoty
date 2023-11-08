package websocket_client

import (
	"context"
	"github.com/opennoty/opennoty/api/api_proto"
	"github.com/opennoty/opennoty/pkg/test_util"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSubscribeAndUnsubscribe(t *testing.T) {
	s, c, cleanup := test_util.MakePair(t)
	defer cleanup()

	ch := make(chan int, 1)
	var subscribeState int = 0

	go func() {
		var payload api_proto.Payload
		var respPayload *api_proto.Payload

		for {
			if err := api_proto.ApiPayloadCodec.Receive(s, &payload); err != nil {
				t.Error(err)
				return
			}

			switch subscribeState {
			case 0:
				assert.Equal(t, api_proto.PayloadType_kPayloadRequest, payload.Type)
				assert.Equal(t, api_proto.RequestMethod_kRequestTopicSubscribe, payload.RequestMethod)

				respPayload = api_proto.NewResponsePayload(&payload)
				respPayload.SubscribeKey = "ABCD"
				if err := api_proto.ApiPayloadCodec.Send(s, respPayload); err != nil {
					t.Error(err)
					return
				}
				ch <- 1
				subscribeState = 1
			case 4:
				assert.Equal(t, api_proto.PayloadType_kPayloadRequest, payload.Type)
				assert.Equal(t, api_proto.RequestMethod_kRequestTopicUnsubscribe, payload.RequestMethod)

				subscribeState = 5

				respPayload = api_proto.NewResponsePayload(&payload)
				respPayload.SubscribeKey = "ABCD"
				if err := api_proto.ApiPayloadCodec.Send(s, respPayload); err != nil {
					t.Error(err)
					return
				}
				ch <- 1
			default:
				t.Error("invalid state")
				break
			}

		}
	}()

	client := NewClient(c)

	subscribeKey1, err := client.Subscribe(context.Background(), "hello", func(topic string, data []byte, subscribeKey string) {

	})
	if err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, 1, <-ch)
	assert.Equal(t, 1, subscribeState)
	subscribeState = 2

	subscribeKey2, err := client.Subscribe(context.Background(), "hello", func(topic string, data []byte, subscribeKey string) {

	})
	if err != nil {
		t.Error(err)
		return
	}

	subscribeState = 3

	client.Unsubscribe(subscribeKey2)

	subscribeState = 4

	client.Unsubscribe(subscribeKey1)

	assert.Equal(t, 5, subscribeState)

	_ = subscribeKey1
	_ = subscribeKey2
}
