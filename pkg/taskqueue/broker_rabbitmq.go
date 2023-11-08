package taskqueue

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/opennoty/opennoty/pkg/health"
	"github.com/opennoty/opennoty/pkg/waitsignal"
	amqp "github.com/rabbitmq/amqp091-go"
	"sync"
)

const (
	amqpDirect = "amq.direct"
)

type amqpClient struct {
	url              string
	config           amqp.Config
	routingKeyPrefix string

	lock     sync.RWMutex
	conn     *amqp.Connection
	lastErr  error
	shutdown bool

	connSig *waitsignal.Holder[*amqp.Connection]
}

type amqpBroker struct {
	c          *amqpClient
	routingKey string
	queue      amqp.Queue
	mutex      sync.Mutex

	ch *amqp.Channel
}

type AmqpMessage struct {
	Delivery amqp.Delivery
}

func NewAmqpClient(url string, config amqp.Config, routingKeyPrefix string) Client {
	c := &amqpClient{
		url:              url,
		config:           config,
		routingKeyPrefix: routingKeyPrefix,
		connSig:          &waitsignal.Holder[*amqp.Connection]{},
	}
	go c.reconnect()
	return c
}

func (c *amqpClient) Broker(routingKey string) (Broker, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.conn == nil {
		return nil, amqp.ErrClosed
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, err
	}

	routingKey = c.routingKeyPrefix + routingKey

	queue, err := ch.QueueDeclare(routingKey, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	err = ch.QueueBind(queue.Name, routingKey, amqpDirect, false, nil)
	if err != nil {
		return nil, err
	}

	return &amqpBroker{
		c:          c,
		ch:         ch,
		routingKey: routingKey,
		queue:      queue,
	}, err
}

func (c *amqpClient) Shutdown() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.shutdown = true
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

func (c *amqpClient) reconnect() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.conn = nil
	c.connSig.Reset()

	if c.shutdown {
		c.connSig.Throw(errors.New(""))
		return
	}

	conn, err := amqp.DialConfig(c.url, c.config)
	if err != nil {
		c.lastErr = err
		c.connSig.Throw(err)
		return
	}
	c.conn = conn
	c.connSig.Signal(conn)

	closeCh := make(chan *amqp.Error, 1)
	go func() {
		closeErr := <-conn.NotifyClose(closeCh)
		if closeErr != nil {
			c.lastErr = closeErr
		}
		c.reconnect()
	}()
}

func (c *amqpClient) Check() health.Health {
	resp := health.Health{
		Status: health.StatusUp,
	}

	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.conn == nil {
		resp.Status = health.StatusDown
		if c.lastErr != nil {
			resp.Reason = c.lastErr.Error()
		}
	}
	return resp
}

func (b *amqpBroker) getChannel() (*amqp.Channel, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.ch.IsClosed() {
		b.ch = nil

		conn, err := b.c.connSig.Wait()
		if err != nil {
			return nil, err
		}

		ch, err := conn.Channel()
		if err != nil {
			return nil, err
		}
		b.ch = ch
	}

	return b.ch, nil
}

func (b *amqpBroker) Publish(body interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	ch, err := b.getChannel()
	if err != nil {
		return err
	}
	p := amqp.Publishing{
		Body: raw,
	}
	return ch.PublishWithContext(context.Background(), amqpDirect, b.routingKey, false, false, p)
}

func (b *amqpBroker) Consume(ctx context.Context) (chan Message, error) {
	ch, err := b.getChannel()
	if err != nil {
		return nil, err
	}

	deliveryCh, err := ch.ConsumeWithContext(ctx, b.queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return nil, err
	}

	messageCh := make(chan Message, 1)
	go func() {
		closeCh := make(chan *amqp.Error, 1)
		ch.NotifyClose(closeCh)

	loop:
		for ctx.Err() == nil {
			select {
			case d := <-deliveryCh:
				messageCh <- &AmqpMessage{
					Delivery: d,
				}
				break
			case <-closeCh:
				ch, err := b.getChannel()
				if err != nil {
					close(messageCh)
					break loop
				}

				deliveryCh, err = ch.ConsumeWithContext(ctx, b.queue.Name, "", false, false, false, false, nil)
				if err != nil {
					close(messageCh)
					break loop
				}
				break
			}
		}
	}()

	return messageCh, nil
}

func (b *amqpBroker) Ack(msg Message) error {
	return b.ack(msg, true, false)
}

func (b *amqpBroker) Nack(msg Message, requeue bool) error {
	return b.ack(msg, false, requeue)
}

func (b *amqpBroker) ack(msg Message, ack bool, requeue bool) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.ch == nil {
		return amqp.ErrClosed
	}

	amqpMessage := (msg).(*AmqpMessage)
	if ack {
		return b.ch.Ack(amqpMessage.Delivery.DeliveryTag, false)
	} else {
		return b.ch.Nack(amqpMessage.Delivery.DeliveryTag, false, true)
	}
}

func (m *AmqpMessage) Body() []byte {
	return m.Delivery.Body
}

func (m *AmqpMessage) Parse(jsonObj interface{}) error {
	return json.Unmarshal(m.Delivery.Body, jsonObj)
}
