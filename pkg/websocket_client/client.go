package websocket_client

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

var (
	ErrTimeout = errors.New("timeout")
)

type SubscribeKey = string

type Client struct {
	rawConn *websocket.Conn

	lock                     sync.Mutex
	subscriptionsByTopic     map[string]*subscribeContext
	subscriptionsByServerKey map[SubscribeKey]*subscribeContext
	subscriptionsByLocalKey  map[SubscribeKey]*subscribeHandlerItem

	requestLock     sync.Mutex
	requestContexts map[string]*requestContext
}

type SubscribeHandler func(topic string, data []byte, subscribeKey string)

func Dial(targetUrl string, headers http.Header) (*Client, error) {
	parsedUrl, err := url.Parse(targetUrl)
	if err != nil {
		return nil, err
	}
	scheme := strings.ToLower(parsedUrl.Scheme)
	switch scheme {
	case "http":
		parsedUrl.Scheme = "ws"
		break
	case "https":
		parsedUrl.Scheme = "wss"
		break
	}
	parsedUrl = parsedUrl.JoinPath("api/ws")
	config, err := websocket.NewConfig(parsedUrl.String(), targetUrl)
	if headers != nil {
		config.Header = headers
	}
	conn, err := websocket.DialConfig(config)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}

func NewClient(conn *websocket.Conn) *Client {
	c := &Client{
		rawConn:                  conn,
		requestContexts:          map[string]*requestContext{},
		subscriptionsByTopic:     map[string]*subscribeContext{},
		subscriptionsByServerKey: map[SubscribeKey]*subscribeContext{},
		subscriptionsByLocalKey:  map[SubscribeKey]*subscribeHandlerItem{},
	}
	go c.receiveWorker()
	return c
}

func (c *Client) Close() error {
	return c.rawConn.Close()
}

func (c *Client) getSubscribeContextByTopic(topic string, creatable bool) (*subscribeContext, bool) {
	ctx, ok := c.subscriptionsByTopic[topic]
	if !ok && creatable {
		ctx = &subscribeContext{
			topic:    topic,
			handlers: subscribeHandlers{},
		}
		c.subscriptionsByTopic[topic] = ctx
		return ctx, true
	}
	return ctx, false
}

func (c *Client) Subscribe(ctx context.Context, topic string, handler SubscribeHandler) (SubscribeKey, error) {
	var err error
	var subscribeKey SubscribeKey = uuid.NewString()
	var subscribeCtx *subscribeContext
	var isFirst bool

	handlerItem := &subscribeHandlerItem{
		ctx:          ctx,
		subscribeKey: subscribeKey,
		handler:      handler,
	}

	c.lock.Lock()
	subscribeCtx, isFirst = c.getSubscribeContextByTopic(topic, true)
	handlerItem.subscribeCtx = subscribeCtx
	c.subscriptionsByLocalKey[subscribeKey] = handlerItem
	subscribeCtx.handlers[subscribeKey] = handlerItem
	c.lock.Unlock()

	if isFirst {
		err = c.internalSubscribeRequest(ctx, subscribeCtx, subscribeKey)
		if err != nil {
			return "", err
		}
	}

	return subscribeKey, nil
}

func (c *Client) Unsubscribe(subscribeKey string) error {
	c.lock.Lock()
	finishCh := c.internalUnsubscribe(subscribeKey)
	c.lock.Unlock()
	return <-finishCh
}
