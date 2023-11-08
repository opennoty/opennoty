package api_proto

import (
	"github.com/opennoty/opennoty/pkg/test_util"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestApiPayloadCodec(t *testing.T) {
	s, c, cleanup := test_util.MakePair(t)
	defer cleanup()

	if err := ApiPayloadCodec.Send(s, &Payload{
		Type:          PayloadType_kPayloadRequest,
		RequestMethod: RequestMethod_kRequestTopicSubscribe,
		TopicName:     "hello",
	}); err != nil {
		t.Error(err)
		return
	}

	var receivedPayload Payload
	if err := ApiPayloadCodec.Receive(c, &receivedPayload); err != nil {
		t.Error(err)
		return
	}

	assert.Equal(t, PayloadType_kPayloadRequest, receivedPayload.Type)
	assert.Equal(t, "hello", receivedPayload.TopicName)
}
