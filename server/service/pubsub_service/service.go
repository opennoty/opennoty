package pubsub_service

import (
	"bytes"
	"context"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/hashicorp/go-set"
	"github.com/opennoty/opennoty/pkg/mongo_kvstore"
	"github.com/opennoty/opennoty/server/api/peer_proto"
	"github.com/opennoty/opennoty/server/service/peer_service"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"sync"
	"time"
)

type SubscribeHandlerMap map[SubscribeKey]SubscribeHandler

type SubscribeContext struct {
	topicKey    TopicKey
	doc         SubscriberDocument
	handlers    SubscribeHandlerMap
	docInserted bool
}

func (c *SubscribeContext) cloneHandlers() SubscribeHandlerMap {
	clone := map[SubscribeKey]SubscribeHandler{}
	for key, handler := range c.handlers {
		clone[key] = handler
	}
	return clone
}

type pubSubService struct {
	appCtx                context.Context
	peerService           peer_service.PeerService
	mongoDatabase         *mongo.Database
	subscribersCollection *mongo.Collection

	mongoStore            *mongo_kvstore.MongoKVStore[*SubscriberDocument]
	registerSelfTicker    *time.Ticker
	lock                  sync.RWMutex
	remoteSubscribers     map[TopicKey]*set.Set[PeerId]
	localSubscribers      map[TopicKey]*SubscribeContext
	localSubscribersByKey map[SubscribeKey]*SubscribeContext

	notifyCh chan *notifyData
}

func makeTopicKey(tenantId string, topic string) TopicKey {
	return TopicKey(tenantId + "|" + topic)
}

func NewSubscribeKey() SubscribeKey {
	return SubscribeKey(uuid.NewString())
}

func New(peerService peer_service.PeerService) PubSubService {
	s := &pubSubService{
		peerService: peerService,
		notifyCh:    make(chan *notifyData, 16),
	}
	peerService.RegisterPayloadHandler(peer_proto.PayloadType_kPayloadTopicNotify, s.handleNotifyPayload)
	return s
}

func (s *pubSubService) Start(appCtx context.Context, mongoDatabase *mongo.Database) error {
	var err error

	s.appCtx = appCtx
	s.mongoDatabase = mongoDatabase
	s.subscribersCollection = mongoDatabase.Collection("opennoty.subscriberPeers")

	s.remoteSubscribers = map[TopicKey]*set.Set[PeerId]{}
	s.localSubscribers = map[TopicKey]*SubscribeContext{}
	s.localSubscribersByKey = map[SubscribeKey]*SubscribeContext{}

	if err = s.initializeMongo(); err != nil {
		return err
	}

	s.mongoStore = mongo_kvstore.NewMongoKVStore[*SubscriberDocument]()
	s.mongoStore.UseStore = true
	s.mongoStore.NewDocument = func() *SubscriberDocument {
		return &SubscriberDocument{}
	}
	s.mongoStore.Handler = s
	if err = s.mongoStore.Start(s.appCtx, s.subscribersCollection); err != nil {
		return err
	}

	s.registerSelfTicker = time.NewTicker(time.Second * 10)
	go func() {
		for t := range s.registerSelfTicker.C {
			err := s.registerSelf()
			if err != nil {
				log.Errorf("registerSelf failed: %v", err)
			}
			_ = t
		}
	}()

	go s.notifyWorker()

	return err
}

func (s *pubSubService) Stop() {
	close(s.notifyCh)

	if s.registerSelfTicker != nil {
		s.registerSelfTicker.Stop()
		s.registerSelfTicker = nil
	}

	if s.subscribersCollection != nil {
		_, err := s.subscribersCollection.DeleteMany(context.Background(), bson.M{
			"peerId": s.peerService.GetPeerId(),
		})
		if err != nil {
			log.Errorf("[PubSubService] subscribers deleteMany failed: %v", err)
		}
	}
}

func (s *pubSubService) initializeMongo() error {
	_, err := s.subscribersCollection.Indexes().CreateMany(context.Background(), subscriberIndexes)
	if err != nil {
		return errors.Wrap(err, "[pubSubService] index create failed")
	}
	return nil
}

func (s *pubSubService) OnInserted(doc *SubscriberDocument) {
	if doc.PeerId == s.peerService.GetPeerId() {
		return
	}

	topicKey := doc.TopicKey()
	s.lock.Lock()
	defer s.lock.Unlock()
	targetPeers := s.getRemote(topicKey, true)
	targetPeers.Insert(PeerId(doc.PeerId))

	log.Debugf("targetPeers[OnInserted]: %v", targetPeers.Slice())
}

func (s *pubSubService) OnUpdated(doc *SubscriberDocument) {
}

func (s *pubSubService) OnDeleted(key string, doc *SubscriberDocument) {
	if doc.PeerId == s.peerService.GetPeerId() {
		return
	}

	topicKey := doc.TopicKey()
	s.lock.Lock()
	defer s.lock.Unlock()
	targetPeers := s.getRemote(topicKey, false)
	if targetPeers != nil {
		targetPeers.Remove(PeerId(doc.PeerId))
		if targetPeers.Size() == 0 {
			delete(s.remoteSubscribers, topicKey)
		}
	}

	log.Debugf("targetPeers[OnDeleted]: %v", targetPeers.Slice())
}

func (s *pubSubService) Publish(tenantId string, topic string, data []byte) error {
	var remotePeers []PeerId
	var localHandlers SubscribeHandlerMap

	topicKey := makeTopicKey(tenantId, topic)

	s.lock.RLock()
	local := s.getLocal(topicKey, false)
	if local != nil {
		localHandlers = local.cloneHandlers()
	}

	remote := s.getRemote(topicKey, false)
	if remote != nil {
		remotePeers = remote.Slice()
	}
	s.lock.RUnlock()

	var wg sync.WaitGroup

	if len(remotePeers) > 0 {
		wg.Add(1)
		go func() {
			for _, peerId := range remotePeers {
				if err := s.peerService.NotifyTopic(string(peerId), tenantId, topic, data); err != nil {
					log.Debugf("Publish to remote(%s) failed: %v", peerId, err)
				}
			}
			wg.Done()
		}()
	}

	if localHandlers != nil {
		wg.Add(1)
		go func() {
			for key, handler := range localHandlers {
				if err := handler(tenantId, topic, data, key); err != nil {
					log.Debugf("Publish to local(%s) failed: %v", key, err)
				}
			}
			wg.Done()
		}()
	}

	wg.Wait()

	return nil
}

func (s *pubSubService) Subscribe(tenantId string, topic string, handler SubscribeHandler) SubscribeKey {
	var isFirst bool
	var ctx *SubscribeContext

	key := NewSubscribeKey()
	now := time.Now()

	s.lock.Lock()
	ctx = s.getLocal(makeTopicKey(tenantId, topic), true)
	isFirst = !ctx.docInserted
	ctx.handlers[key] = handler
	s.lock.Unlock()

	if isFirst {
		ctx.doc.Id = primitive.NewObjectID()
		ctx.doc.PeerId = s.peerService.GetPeerId()
		ctx.doc.TenantId = tenantId
		ctx.doc.Topic = topic
		ctx.doc.HeartbeatAt = now
		_, err := s.subscribersCollection.InsertOne(
			context.Background(),
			&ctx.doc,
		)
		if err == nil {
			ctx.docInserted = true
		} else {
			log.Errorf("[PubSubService] subscribers insertOne failed: %v", err)
		}
	}

	return key
}

func (s *pubSubService) Unsubscribe(subscribeKey SubscribeKey) {
	s.Unsubscribes([]SubscribeKey{subscribeKey})
}

func (s *pubSubService) Unsubscribes(subscribeKeys []SubscribeKey) {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, subscribeKey := range subscribeKeys {
		localSubscribers := s.localSubscribersByKey[subscribeKey]
		if localSubscribers == nil {
			return
		}
		delete(localSubscribers.handlers, subscribeKey)
		delete(s.localSubscribersByKey, subscribeKey)

		if len(localSubscribers.handlers) == 0 {
			_, err := s.subscribersCollection.DeleteOne(context.Background(), bson.M{
				"_id": localSubscribers.doc.Id,
			})
			localSubscribers.docInserted = false
			delete(s.localSubscribers, localSubscribers.topicKey)
			if err != nil {
				log.Errorf("[PubSubService] subscribers deleteOne failed: %v", err)
			}
		}
	}
}

func (s *pubSubService) registerSelf() error {
	now := time.Now()
	ctx := context.Background()
	result, err := s.subscribersCollection.UpdateMany(
		ctx,
		bson.M{
			"peerId": s.peerService.GetPeerId(),
		},
		bson.M{
			"$set": bson.M{
				"heartbeatAt": now,
			},
		})
	if err == nil {
		log.Debugf("[PubSubService] registerSelf result: %v", result)
	}
	return err
}

func (s *pubSubService) getRemote(key TopicKey, creatable bool) *set.Set[PeerId] {
	obj, ok := s.remoteSubscribers[key]
	if !ok && creatable {
		obj = set.New[PeerId](128)
		s.remoteSubscribers[key] = obj
	}
	return obj
}

func (s *pubSubService) getLocal(topicKey TopicKey, creatable bool) *SubscribeContext {
	obj, ok := s.localSubscribers[topicKey]
	if !ok && creatable {
		obj = &SubscribeContext{
			topicKey: topicKey,
			handlers: SubscribeHandlerMap{},
		}
		s.localSubscribers[topicKey] = obj
	}
	return obj
}

func (s *pubSubService) handleNotifyPayload(p *peer_service.Peer, payload *peer_proto.Payload) {
	topicKey := makeTopicKey(payload.TenantId, payload.TopicName)
	s.notifyCh <- &notifyData{
		TenantId:  payload.TenantId,
		TopicKey:  topicKey,
		TopicName: payload.TopicName,
		TopicData: bytes.Clone(payload.TopicData),
	}
}

func (s *pubSubService) notifyWorker() {
	for {
		notifyItem, ok := <-s.notifyCh
		if !ok {
			break
		}

		var handlers SubscribeHandlerMap

		s.lock.RLock()
		subscribers, ok := s.localSubscribers[notifyItem.TopicKey]
		if ok {
			handlers = subscribers.cloneHandlers()
		}
		s.lock.RUnlock()

		handlers.invokeAll(notifyItem.TenantId, notifyItem.TopicName, notifyItem.TopicData)
	}
}

func (m SubscribeHandlerMap) invokeAll(tenantId string, topic string, data []byte) {
	var wg sync.WaitGroup

	for key, handler := range m {
		wg.Add(1)

		go func(key SubscribeKey, handler SubscribeHandler) {
			if err := handler(tenantId, topic, data, key); err != nil {
				log.Debugf("[PubSubService] invokeAll failed at %s: %v", key, err)
			}
			wg.Done()
		}(key, handler)
	}

	wg.Wait()
}

type notifyData struct {
	TenantId  string
	TopicKey  TopicKey
	TopicName string
	TopicData []byte
}
