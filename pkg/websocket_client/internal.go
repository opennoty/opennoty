package websocket_client

import (
	"context"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/opennoty/opennoty/api/api_proto"
	"google.golang.org/protobuf/proto"
	"io"
	"time"
)

type result struct {
	err  error
	data *api_proto.Payload
}

type requestContext struct {
	id       string
	resultCh chan result
}

type subscribeHandlers map[string]*subscribeHandlerItem

type subscribeContext struct {
	topic              string
	serverSubscribeKey string
	handlers           subscribeHandlers
}

type subscribeHandlerItem struct {
	ctx          context.Context
	subscribeKey SubscribeKey
	subscribeCtx *subscribeContext
	handler      SubscribeHandler
}

func (c *Client) internalSubscribeRequest(ctx context.Context, subscribeCtx *subscribeContext, subscribeKey SubscribeKey) error {
	var err error
	var resp *api_proto.Payload
	requestPayload := api_proto.NewRequestPayload(api_proto.RequestMethod_kRequestTopicSubscribe)
	requestPayload.TopicName = subscribeCtx.topic
	resp, err = c.request(
		ctx,
		requestPayload,
	)

	c.lock.Lock()
	defer c.lock.Unlock()

	if err != nil {
		c.internalUnsubscribe(subscribeKey)
		return err
	}

	subscribeCtx.serverSubscribeKey = resp.SubscribeKey
	c.subscriptionsByServerKey[resp.SubscribeKey] = subscribeCtx
	return nil
}

func (c *Client) internalUnsubscribe(subscribeKey string) chan error {
	finishCh := make(chan error, 1)
	handlerItem := c.subscriptionsByLocalKey[subscribeKey]
	if handlerItem == nil {
		finishCh <- nil
		close(finishCh)
		return finishCh
	}

	subscribeCtx := handlerItem.subscribeCtx
	delete(c.subscriptionsByLocalKey, subscribeKey)
	delete(subscribeCtx.handlers, subscribeKey)

	isLast := len(subscribeCtx.handlers) == 0
	if isLast {
		delete(c.subscriptionsByTopic, subscribeCtx.topic)
		delete(c.subscriptionsByServerKey, subscribeCtx.serverSubscribeKey)
		go func() {
			payload := api_proto.NewRequestPayload(api_proto.RequestMethod_kRequestTopicUnsubscribe)
			payload.TopicName = subscribeCtx.topic
			payload.SubscribeKey = subscribeCtx.serverSubscribeKey
			_, err := c.request(context.Background(), payload)
			finishCh <- err
			close(finishCh)
		}()
	} else {
		finishCh <- nil
		close(finishCh)
	}

	return finishCh
}

func (c *Client) request(ctx context.Context, payload *api_proto.Payload) (*api_proto.Payload, error) {
	if len(payload.RequestId) == 0 {
		payload.RequestId, _ = uuid.New().MarshalBinary()
	}
	requestUuid, err := uuid.FromBytes(payload.RequestId)
	if err != nil {
		return nil, err
	}
	requestId := requestUuid.String()

	payload.Type = api_proto.PayloadType_kPayloadRequest
	reqCtx := &requestContext{
		id:       requestId,
		resultCh: make(chan result, 1),
	}

	c.requestLock.Lock()
	c.requestContexts[requestId] = reqCtx
	err = api_proto.ApiPayloadCodec.Send(c.rawConn, payload)
	c.requestLock.Unlock()
	if err != nil {
		return nil, err
	}

	defer func() {
		c.requestLock.Lock()
		delete(c.requestContexts, requestId)
		c.requestLock.Unlock()
	}()

	timeoutCtx, _ := context.WithTimeout(ctx, time.Second)

	select {
	case <-timeoutCtx.Done():
		return nil, timeoutCtx.Err()
	case res := <-reqCtx.resultCh:
		if res.err != nil {
			return nil, res.err
		} else {
			return res.data, nil
		}
	}
}

func (c *Client) receiveWorker() {
	var receivedPayload api_proto.Payload

	for {
		receivedPayload.Reset()
		err := api_proto.ApiPayloadCodec.Receive(c.rawConn, &receivedPayload)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Warnf("error %v", err)
			break
		}

		switch receivedPayload.Type {
		case api_proto.PayloadType_kPayloadResponse:
			c.handleResponse(&receivedPayload)
		case api_proto.PayloadType_kPayloadTopicNotify:
			c.handleTopicNotify(&receivedPayload)
		}
	}
}

func (c *Client) handleResponse(payload *api_proto.Payload) error {
	requestId, err := uuid.FromBytes(payload.RequestId)
	if err != nil {
		return err
	}
	requestIdStr := requestId.String()

	c.requestLock.Lock()
	reqCtx, ok := c.requestContexts[requestIdStr]
	c.requestLock.Unlock()

	if !ok {
		return nil
	}

	reqCtx.resultCh <- result{
		err:  nil,
		data: proto.Clone(payload).(*api_proto.Payload),
	}

	return nil
}

func (c *Client) handleTopicNotify(payload *api_proto.Payload) error {
	topicName := payload.TopicName
	topicData := payload.TopicData

	c.lock.Lock()
	subscribeCtx, ok := c.subscriptionsByTopic[payload.TopicName]
	if !ok {
		c.lock.Unlock()
		return nil
	}
	handlers := subscribeCtx.handlers.cloneHandlers()
	c.lock.Unlock()

	handlers.invokeAll(topicName, topicData)

	return nil
}

func (m subscribeHandlers) cloneHandlers() subscribeHandlers {
	clone := map[string]*subscribeHandlerItem{}
	for key, handler := range m {
		clone[key] = handler
	}
	return clone
}

func (m subscribeHandlers) invokeAll(topic string, data []byte) {
	for key, item := range m {
		item.handler(topic, data, key)
	}
}
