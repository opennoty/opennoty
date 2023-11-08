package peer_transport

import (
	"github.com/opennoty/opennoty/server/api/peer_proto"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
)

func TestReadWrite(t *testing.T) {
	var payload peer_proto.Payload
	payload.TopicName = "HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456HELLOWORLD123456"

	r, w := io.Pipe()
	reader := NewReader(r)

	go func() {
		for _ = range [10]int{} {
			if err := WritePayload(w, &payload); err != nil {
				t.Error(err)
			}
		}
	}()

	for _ = range [10]int{} {
		readPayload, err := reader.Read()
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, payload.TopicName, readPayload.TopicName)
	}
}
