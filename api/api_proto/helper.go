package api_proto

import (
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
	"google.golang.org/protobuf/proto"
)

func NewRequestPayload(requestMethod RequestMethod) *Payload {
	requestId := uuid.New()
	requestIdRaw, _ := requestId.MarshalBinary()
	return &Payload{
		Type:          PayloadType_kPayloadRequest,
		RequestId:     requestIdRaw,
		RequestMethod: requestMethod,
	}
}

func NewResponsePayload(requestPayload *Payload) *Payload {
	return &Payload{
		Type:          PayloadType_kPayloadResponse,
		RequestMethod: requestPayload.RequestMethod,
		RequestId:     requestPayload.RequestId,
	}
}

var ApiPayloadCodec = websocket.Codec{payloadMarshal, payloadUnmarshal}

func payloadMarshal(v interface{}) (msg []byte, payloadType byte, err error) {
	msg, err = proto.Marshal(v.(proto.Message))
	return msg, websocket.BinaryFrame, err
}

func payloadUnmarshal(msg []byte, payloadType byte, v interface{}) (err error) {
	if payloadType != websocket.BinaryFrame {
		return nil
	}
	return proto.Unmarshal(msg, v.(proto.Message))
}
