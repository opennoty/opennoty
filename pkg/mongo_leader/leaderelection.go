package mongo_leader

import (
	"context"
	"github.com/gofiber/fiber/v2/log"
	"github.com/opennoty/opennoty/pkg/mongo_util"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type NotifyFunc func(result ElectionResult)

type LeaderElection struct {
	leaderCollection *mongo.Collection
	resultCh         chan ElectionResult
	ctx              context.Context
	cancel           func()

	key         string
	currentRole Role
	peerId      string

	notifyFunctions []NotifyFunc
}

type Role string

const (
	RoleLeader   Role = "leader"
	RoleFollower Role = "follower"
)

type ElectionResult struct {
	MyRole       Role
	LeaderPeerId string
}

func New() *LeaderElection {
	ctx, cancel := context.WithCancel(context.Background())

	return &LeaderElection{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (l *LeaderElection) Start(mongoDatabase *mongo.Database, collectionName string, key string, peerId string) error {
	l.leaderCollection = mongoDatabase.Collection(collectionName)
	l.key = key
	l.peerId = peerId

	if err := l.initializeMongo(); err != nil {
		return err
	}

	go l.worker()

	return nil
}

func (l *LeaderElection) Stop() {
	l.cancel()
}

func (l *LeaderElection) AddNotify(fn NotifyFunc) {
	l.notifyFunctions = append(l.notifyFunctions, fn)
}

func (l *LeaderElection) initializeMongo() error {
	_, err := l.leaderCollection.Indexes().CreateMany(context.Background(), leaderIndexes)
	if err != nil {
		return errors.Wrap(err, "[LeaderElection] index create failed")
	}
	return nil
}

func (l *LeaderElection) worker() {
	ticker := time.NewTicker(time.Second * 5)
	done := l.ctx.Done()
loop:
	for l.ctx.Err() == nil {
		select {
		case <-ticker.C:
			l.refresh()
			break
		case <-done:
			break loop
		}
	}
	close(l.resultCh)
}

func (l *LeaderElection) refresh() {
	var nextRole Role
	var document LeaderDocument

	heartbeatAt := time.Now()

	err := mongo_util.WithSession(l.leaderCollection.Database(), context.Background(), func(sc mongo.SessionContext) error {
		result := l.leaderCollection.FindOneAndUpdate(sc, &bson.M{
			"key":    l.key,
			"peerId": l.peerId,
		}, &bson.M{
			"$setOnInsert": bson.M{
				"_id":    primitive.NewObjectID(),
				"key":    l.key,
				"peerId": l.peerId,
			},
			"$set": bson.M{
				"heartbeatAt": heartbeatAt,
			},
		}, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After))
		err := result.Err()
		if err == nil {
			nextRole = RoleLeader
			return result.Decode(&document)
		} else {
			cmdErr, ok := err.(mongo.CommandError)
			if ok && cmdErr.HasErrorCode(11000) {
				nextRole = RoleFollower
				result = l.leaderCollection.FindOne(sc, &bson.M{
					"key": l.key,
				})
				if err = result.Err(); err != nil {
					return err
				}
				return result.Decode(&document)
			}
		}
		return err
	})
	if err != nil {
		log.Warnf("[LeaderElection] refresh failed: %v", err)
	} else {
		if l.currentRole != nextRole {
			l.currentRole = nextRole
			var result = ElectionResult{
				MyRole:       nextRole,
				LeaderPeerId: document.PeerId,
			}
			for _, fn := range l.notifyFunctions {
				fn(result)
			}
		}
	}
}
