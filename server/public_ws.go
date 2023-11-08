package server

import (
	"errors"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set"
	"github.com/opennoty/opennoty/api/api_proto"
	"github.com/opennoty/opennoty/server/service/notification_service"
	"github.com/opennoty/opennoty/server/service/pubsub_service"
	"google.golang.org/protobuf/proto"
	"net"
	"sync"
	"sync/atomic"
)

type WebSocketRequestHandler func(c *WebSocketConnection, payload *api_proto.Payload) (*api_proto.Payload, error)

type WebSocketConnection struct {
	clientId   string
	rawConn    *websocket.Conn
	remoteAddr net.Addr

	lock          sync.RWMutex
	connected     int32
	clientContext *ClientContext

	readPayloadBuffer api_proto.Payload

	subscribeKeys *set.Set[pubsub_service.SubscribeKey]

	writeCh chan *api_proto.Payload
}

func (s *Server) websocketHandler(rawConn *websocket.Conn) {
	conn := &WebSocketConnection{
		clientId:   uuid.NewString(),
		rawConn:    rawConn,
		remoteAddr: rawConn.RemoteAddr(),

		subscribeKeys: set.New[pubsub_service.SubscribeKey](8),
		writeCh:       make(chan *api_proto.Payload, 1),
		connected:     1,
		clientContext: rawConn.Locals(clientContextKey).(*ClientContext),
	}

	if s.option.WebsocketInterceptor != nil {
		err := s.option.WebsocketInterceptor(conn)
		if err != nil {
			log.Warnf("WebSocket[%s, %s] rejected: %v", conn.clientId, conn.remoteAddr.String(), err)
		}
	}

	log.Debugf("WebSocket[%s, %s] connected", conn.clientId, conn.remoteAddr.String())

	go func() {
		for atomic.LoadInt32(&conn.connected) == 1 {
			payload, ok := <-conn.writeCh
			if !ok {
				break
			}
			msg, err := proto.Marshal(payload)
			if err != nil {
				log.Warnf("WebSocket[%s, %s] proto marshal failed: %v", conn.clientId, conn.remoteAddr.String(), err)
			} else {
				if err = conn.rawConn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
					log.Warnf("WebSocket[%s, %s] write failed: %v", conn.clientId, conn.remoteAddr.String(), err)
					break
				}
			}
		}
	}()

	for {
		payload, err := conn.readPayload()
		if err != nil {
			log.Warnf("WebSocket[%s, %s] read error: %v", conn.clientId, conn.remoteAddr.String(), err)
			break
		}

		handler := s.websocketRequestHandlers[payload.RequestMethod]
		if handler != nil {
			responsePayload, err := handler(conn, payload)
			if err != nil {
				log.Warnf("WebSocket[%s, %s] handle error: %v", conn.clientId, conn.remoteAddr.String(), err)
				break
			}
			if responsePayload != nil {
				conn.writeCh <- responsePayload
			}
		}
	}

	conn.lock.Lock()
	atomic.StoreInt32(&conn.connected, 0)
	close(conn.writeCh)
	conn.writeCh = nil
	conn.lock.Unlock()

	s.websocketClosed(conn)
}

func (s *Server) websocketClosed(c *WebSocketConnection) {
	s.pubSubService.Unsubscribes(c.subscribeKeys.Slice())
}

func (s *Server) websocketSubscribe(c *WebSocketConnection, requestPayload *api_proto.Payload) (*api_proto.Payload, error) {
	clientContext := c.ClientContext()
	responsePayload := NewApiResponsePayload(requestPayload)
	if clientContext.CheckSubscribeTopic(requestPayload.TopicName) {
		subscribeKey := s.pubSubService.Subscribe(c.clientContext.GetTenantId(), requestPayload.TopicName, c.subscribeHandler)
		c.subscribeKeys.Insert(subscribeKey)
		responsePayload.ResponseOk = true
		responsePayload.SubscribeKey = string(subscribeKey)
	} else {
		responsePayload.ResponseOk = false
	}
	return responsePayload, nil
}

func (s *Server) websocketUnsubscribe(c *WebSocketConnection, requestPayload *api_proto.Payload) (*api_proto.Payload, error) {
	subscribeKey := pubsub_service.SubscribeKey(requestPayload.SubscribeKey)

	if c.subscribeKeys.Contains(subscribeKey) {
		s.pubSubService.Unsubscribe(subscribeKey)
		c.subscribeKeys.Remove(subscribeKey)
	}

	responsePayload := NewApiResponsePayload(requestPayload)
	responsePayload.ResponseOk = true
	return responsePayload, nil
}

func (s *Server) websocketStartNotification(c *WebSocketConnection, requestPayload *api_proto.Payload) (*api_proto.Payload, error) {
	clientContext := c.ClientContext()
	responsePayload := NewApiResponsePayload(requestPayload)

	subscribeKey := s.pubSubService.Subscribe(clientContext.GetTenantId(), notification_service.NotificationTopicName(clientContext.GetAccountId()), c.subscribeHandler)
	c.subscribeKeys.Insert(subscribeKey)

	responsePayload.ResponseOk = true
	responsePayload.SubscribeKey = string(subscribeKey)

	return responsePayload, nil
}

func (s *Server) websocketFetchNotifications(c *WebSocketConnection, requestPayload *api_proto.Payload) (*api_proto.Payload, error) {
	clientContext := c.ClientContext()
	requestParams := requestPayload.FetchNotificationRequest

	responsePayload := NewApiResponsePayload(requestPayload)
	responsePayload.FetchNotificationResponse = &api_proto.FetchNotificationsResponse{}

	limits := int(requestParams.GetLimits())
	if limits <= 0 {
		limits = 10
	} else if limits > 100 {
		limits = 100
	}

	documents, err := s.notificationService.GetNotifications(clientContext.GetTenantId(), clientContext.GetAccountId(), &notification_service.NotificationFilter{
		ContinueToken: requestParams.GetContinueToken(),
		Limits:        limits,
	})
	if err == nil {
		responsePayload.ResponseOk = true
		if len(documents) > 0 {
			responsePayload.FetchNotificationResponse.ContinueToken = documents[len(documents)-1].Id.Hex()
		}
		for _, document := range documents {
			notiJson, err := notification_service.NotificationDocToJson(document)
			if err != nil {
				break
			}
			responsePayload.FetchNotificationResponse.Item = append(responsePayload.FetchNotificationResponse.Item, string(notiJson))
		}
	}
	if err != nil {
		responsePayload.ResponseOk = false
	}

	return responsePayload, nil
}

func (s *Server) websocketMarkNotifications(c *WebSocketConnection, requestPayload *api_proto.Payload) (*api_proto.Payload, error) {
	clientContext := c.ClientContext()
	requestParams := requestPayload.MarkNotificationsRequest

	responsePayload := NewApiResponsePayload(requestPayload)

	err := s.notificationService.MarkNotifications(clientContext.GetTenantId(), clientContext.GetAccountId(), requestParams)
	if err == nil {
		responsePayload.ResponseOk = true
	} else {
		responsePayload.ResponseOk = false
	}

	return responsePayload, nil
}

func NewApiResponsePayload(requestPayload *api_proto.Payload) *api_proto.Payload {
	return &api_proto.Payload{
		Type:          api_proto.PayloadType_kPayloadResponse,
		RequestMethod: requestPayload.RequestMethod,
		RequestId:     requestPayload.RequestId,
	}
}

func (c *WebSocketConnection) RawConn() *websocket.Conn {
	return c.rawConn
}

func (c *WebSocketConnection) ClientContext() *ClientContext {
	return c.clientContext
}

func (c *WebSocketConnection) readPayload() (*api_proto.Payload, error) {
	var (
		msg []byte
		err error
	)
	if _, msg, err = c.rawConn.ReadMessage(); err != nil {
		return nil, err
	}
	if err = proto.Unmarshal(msg, &c.readPayloadBuffer); err != nil {
		return nil, err
	}
	return &c.readPayloadBuffer, nil
}

func (c *WebSocketConnection) subscribeHandler(tenantId string, topic string, data []byte, subscribeKey pubsub_service.SubscribeKey) error {
	c.lock.RLock()
	defer c.lock.RUnlock()
	if c.writeCh == nil {
		return errors.New("closed connection")
	}
	c.writeCh <- &api_proto.Payload{
		Type:      api_proto.PayloadType_kPayloadTopicNotify,
		TopicName: topic,
		TopicData: data,
	}
	return nil
}
