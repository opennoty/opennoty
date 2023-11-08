package peer_transport

import (
	"fmt"
	"github.com/opennoty/opennoty/server/api/peer_proto"
	"google.golang.org/protobuf/proto"
	"io"
)

// protocol
// [      00] : VERSION (01)
// [01 02 03] : Big-endian payload size

type Reader struct {
	conn         io.Reader
	payloadProto peer_proto.Payload
}

func NewReader(conn io.Reader) *Reader {
	return &Reader{
		conn: conn,
	}
}

func (r *Reader) Read() (*peer_proto.Payload, error) {
	var header [4]byte
	var version byte
	var payloadSize uint

	_, err := io.ReadFull(r.conn, header[:])
	if err != nil {
		return nil, err
	}

	version = header[0]
	payloadSize = uint(header[1]) << 16
	payloadSize |= uint(header[2]) << 8
	payloadSize |= uint(header[3])

	if version != 1 {
		return nil, fmt.Errorf("incompatible protocol version: %d", version)
	}

	payloadRaw := make([]byte, payloadSize-4)
	if _, err = io.ReadFull(r.conn, payloadRaw); err != nil {
		return nil, err
	}

	if err = proto.Unmarshal(payloadRaw, &r.payloadProto); err != nil {
		return nil, err
	}

	return &r.payloadProto, nil
}
