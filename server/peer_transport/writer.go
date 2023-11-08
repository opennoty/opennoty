package peer_transport

import (
	"github.com/opennoty/opennoty/server/api/peer_proto"
	"google.golang.org/protobuf/proto"
	"io"
)

func WritePayload(conn io.Writer, payload *peer_proto.Payload) error {
	var header [4]byte
	var totalSize uint
	var packet []byte

	payloadRaw, err := proto.Marshal(payload)
	if err != nil {
		return err
	}
	totalSize = uint(len(payloadRaw)) + 4

	header[0] = 1
	header[1] = byte(totalSize >> 16)
	header[2] = byte(totalSize >> 8)
	header[3] = byte(totalSize)

	packet = append(header[:], payloadRaw...)
	_, err = conn.Write(packet)

	return err
}
