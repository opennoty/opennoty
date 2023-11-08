package notification_service

import (
	"context"
	"encoding/json"
	"github.com/gofiber/fiber/v2/log"
	"github.com/opennoty/opennoty/api/api_model"
	"github.com/opennoty/opennoty/api/api_proto"
	"github.com/opennoty/opennoty/pkg/mongo_util"
	"github.com/opennoty/opennoty/server/service/pubsub_service"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

// notification is like MQTT QOS Level 1

type notificationService struct {
	appCtx        context.Context
	pubSubService pubsub_service.PubSubService

	mongoDatabase          *mongo.Database
	notificationCollection *mongo.Collection
}

func New(pubSubService pubsub_service.PubSubService) NotificationService {
	return &notificationService{
		pubSubService: pubSubService,
	}
}

func (s *notificationService) Start(appCtx context.Context, mongoDatabase *mongo.Database) error {
	s.mongoDatabase = mongoDatabase
	s.notificationCollection = mongoDatabase.Collection("opennoty.notifications")

	if err := s.initializeMongo(); err != nil {
		return err
	}

	return nil
}

func (s *notificationService) initializeMongo() error {
	var err error

	_, err = s.notificationCollection.Indexes().CreateMany(context.Background(), notificationIndexes)
	if err != nil {
		return errors.Wrap(err, "[NotificationService] notification index create failed")
	}

	return nil
}

func (s *notificationService) CreateNotification(document *NotificationDocument) error {
	if err := mongo_util.WithTransaction(s.mongoDatabase, context.Background(), func(sc mongo.SessionContext) error {
		insertResult, err := s.notificationCollection.InsertOne(sc, document)
		if err == nil {
			document.Id = mongo_util.InsertedIDToObjectId(insertResult.InsertedID)
		}
		return err
	}); err != nil {
		return err
	}

	notiJson, err := NotificationDocToJson(document)
	if err != nil {
		return err
	}

	if err = s.pubSubService.Publish(document.TenantId, NotificationTopicName(document.AccountId), notiJson); err != nil {
		log.Warnf("publish failed: notificationId=%s: %v", document.GetId(), err)
	}
	return nil
}

func (s *notificationService) GetNotifications(tenantId string, accountId string, filter *NotificationFilter) ([]*NotificationDocument, error) {
	var filterBson = bson.M{
		"tenantId":  tenantId,
		"accountId": accountId,
	}
	if filter.ContinueToken != "" {
		nextId, err := primitive.ObjectIDFromHex(filter.ContinueToken)
		if err != nil {
			return nil, err
		}
		filterBson["_id"] = bson.M{
			"$lt": nextId,
		}
	}

	var documents []*NotificationDocument
	err := mongo_util.WithSession(s.mongoDatabase, context.Background(), func(sc mongo.SessionContext) error {
		cursor, err := s.notificationCollection.Find(
			context.Background(),
			filterBson,
			options.Find().SetSort(bson.M{
				"_id": -1,
			}).SetLimit(int64(filter.Limits)),
		)
		if err != nil {
			return err
		}
		defer cursor.Close(sc)
		for cursor.Next(sc) {
			doc := new(NotificationDocument)
			if err = cursor.Decode(doc); err != nil {
				break
			}
			documents = append(documents, doc)
		}
		return err
	})
	return documents, err
}

func (s *notificationService) MarkNotifications(tenantId string, accountId string, params *api_proto.MarkNotificationsRequest) error {
	var err error
	var markReadIds []primitive.ObjectID = make([]primitive.ObjectID, len(params.MarkReadIds))
	var markUnreadIds []primitive.ObjectID = make([]primitive.ObjectID, len(params.UnmarkReadIds))
	var markDeleteIds []primitive.ObjectID = make([]primitive.ObjectID, len(params.DeleteIds))

	for i, id := range params.MarkReadIds {
		markReadIds[i], err = primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
	}
	for i, id := range params.UnmarkReadIds {
		markUnreadIds[i], err = primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
	}
	for i, id := range params.DeleteIds {
		markDeleteIds[i], err = primitive.ObjectIDFromHex(id)
		if err != nil {
			return err
		}
	}

	return mongo_util.WithTransaction(s.mongoDatabase, context.Background(), func(sc mongo.SessionContext) error {
		if len(markReadIds) > 0 {
			res, err := s.notificationCollection.UpdateMany(sc, bson.M{
				"tenantId":  tenantId,
				"accountId": accountId,
				"_id": bson.M{
					"$in": markReadIds,
				},
			}, bson.M{
				"$set": bson.M{
					"readMarked": true,
				},
			})
			if err != nil {
				return err
			}
			_ = res
		}
		if len(markUnreadIds) > 0 {
			res, err := s.notificationCollection.UpdateMany(sc, bson.M{
				"tenantId":  tenantId,
				"accountId": accountId,
				"_id": bson.M{
					"$in": markUnreadIds,
				},
			}, bson.M{
				"$set": bson.M{
					"readMarked": false,
				},
			})
			if err != nil {
				return err
			}
			_ = res
		}
		if len(markDeleteIds) > 0 {
			res, err := s.notificationCollection.UpdateMany(sc, bson.M{
				"tenantId":  tenantId,
				"accountId": accountId,
				"_id": bson.M{
					"$in": markDeleteIds,
				},
			}, bson.M{
				"$set": bson.M{
					"deleted": true,
				},
			})
			if err != nil {
				return err
			}
			_ = res
		}
		return nil
	})
}

func NotificationTopicName(accountId string) string {
	return "opennoty$/account/" + accountId + "/notification"
}

func EventsBsonToJson(input []bsoncore.Document) ([]json.RawMessage, error) {
	var eventsJson []json.RawMessage
	for _, doc := range input {
		eventBson, err := bson.Marshal(doc)
		if err != nil {
			return nil, err
		}

		var eventGo map[string]interface{}
		if err = bson.Unmarshal(eventBson, &eventGo); err != nil {
			return nil, err
		}
		eventJson, err := json.Marshal(eventGo)
		if err != nil {
			return nil, err
		}
		eventsJson = append(eventsJson, eventJson)
	}
	return eventsJson, nil
}

func NotificationDocToJson(document *NotificationDocument) ([]byte, error) {
	var err error
	var notiItem api_model.Notification

	notiItem.Id = document.Id.Hex()
	notiItem.Subject = document.Subject
	notiItem.ReadMarked = document.ReadMarked
	notiItem.Deleted = document.Deleted
	notiItem.Step.TotalCount = document.Step.TotalCount
	notiItem.Step.Digest = document.Step.Digest
	notiItem.Step.Events, err = EventsBsonToJson(document.Step.Events)
	if err != nil {
		return nil, err
	}

	return json.Marshal(&notiItem)
}
