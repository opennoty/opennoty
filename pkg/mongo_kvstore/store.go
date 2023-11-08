package mongo_kvstore

import (
	"context"
	"github.com/gofiber/fiber/v2/log"
	"github.com/opennoty/opennoty/pkg/mongo_util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sync"
)

//TODO: find 시 한번에 lock 걸고

type DocumentLike interface {
	GetId() string
}

type Handler[D DocumentLike] interface {
	OnInserted(doc D)
	OnUpdated(doc D)
	OnDeleted(key string, doc D)
}

type NewDocument[D DocumentLike] func() D
type FindByObjectId[D DocumentLike] func(store map[string]D, id primitive.ObjectID) (string, D)

type MongoKVStore[D DocumentLike] struct {
	UseStore       bool
	Handler        Handler[D]
	NewDocument    NewDocument[D]
	FindByObjectId FindByObjectId[D]

	ctx        context.Context
	cancel     context.CancelFunc
	collection *mongo.Collection

	mongoWatchResume interface{}

	Lock  sync.RWMutex
	Store map[string]D
}

func NewMongoKVStore[D DocumentLike]() *MongoKVStore[D] {
	return &MongoKVStore[D]{
		FindByObjectId: func(store map[string]D, id primitive.ObjectID) (string, D) {
			key := id.String()
			doc, _ := store[key]
			return id.String(), doc
		},
	}
}

func (s *MongoKVStore[D]) Start(ctx context.Context, collection *mongo.Collection) error {
	var err error

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.collection = collection
	s.Store = map[string]D{}

	if err = s.startWatch(); err != nil {
		return err
	}
	if err = s.readAll(); err != nil {
		return err
	}

	return nil
}

func (s *MongoKVStore[D]) Stop() {
	s.cancel()
}

func (s *MongoKVStore[D]) startWatch() error {
	var initialFinished bool = false
	initialResult := make(chan error, 1)

	go func() {
		for s.ctx.Err() == nil {
			option := options.ChangeStream().SetFullDocument(options.UpdateLookup)
			if s.mongoWatchResume != nil {
				option = option.SetResumeAfter(s.mongoWatchResume)
			}
			cursor, err := s.collection.Watch(s.ctx, []any{}, option)
			if err != nil {
				log.Errorf("MongoKVStore: watch failed: %v", err)
				if !initialFinished {
					initialFinished = true
					initialResult <- err
					close(initialResult)
				}
				return
			}

			log.Debugw("MongoKVStore: watch started")

			if !initialFinished {
				initialFinished = true
				initialResult <- nil
				close(initialResult)
			}

			var event mongo_util.WatchChangeEvent[D]
			for cursor.Next(context.TODO()) {
				if err := cursor.Decode(&event); err != nil {
					log.Warnf("peer decode failed: %v", err)
					continue
				}

				s.mongoWatchResume = cursor.ResumeToken()

				if event.OperationType == "insert" {
					s.onInserted(event.FullDocument)
				} else if event.OperationType == "delete" {
					s.onDeleted(event.DocumentKey.Id)
				}
			}

			cursor.Close(context.TODO())

			log.Debugf("MongoKVStore: watch finished: %v", err)
		}
	}()
	return <-initialResult
}

func (s *MongoKVStore[D]) readAll() error {
	cursor, err := s.collection.Find(s.ctx, bson.M{})
	if err != nil {
		return err
	}

	doc := s.NewDocument()
	for cursor.Next(context.TODO()) {
		if err := cursor.Decode(doc); err != nil {
			log.Warnf("MongoKVStore: decode failed: %v", err)
			continue
		}

		s.onInserted(doc)
	}

	return nil
}

func (s *MongoKVStore[D]) onInserted(doc D) {
	id := doc.GetId()
	if s.UseStore {
		s.Lock.Lock()
		defer s.Lock.Unlock()
		s.Store[id] = doc
	}
	if s.Handler != nil {
		s.Handler.OnInserted(doc)
	}
}

func (s *MongoKVStore[D]) onUpdated(doc D) {
	id := doc.GetId()
	if s.UseStore {
		s.Lock.Lock()
		defer s.Lock.Unlock()
		s.Store[id] = doc
	}
	if s.Handler != nil {
		s.Handler.OnUpdated(doc)
	}
}

func (s *MongoKVStore[D]) onDeleted(id primitive.ObjectID) {
	var key string
	var doc D

	s.Lock.Lock()

	key, doc = s.FindByObjectId(s.Store, id)
	if key == "" {
		s.Lock.Unlock()
		return
	}

	if s.UseStore {
		delete(s.Store, key)
	}

	s.Lock.Unlock()

	if s.Handler != nil {
		s.Handler.OnDeleted(key, doc)
	}
}
