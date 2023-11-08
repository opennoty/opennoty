package workflow_service

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2/log"
	"github.com/opennoty/opennoty/api/api_model"
	"github.com/opennoty/opennoty/api/internal_bson"
	"github.com/opennoty/opennoty/pkg/mailsender"
	"github.com/opennoty/opennoty/pkg/mongo_leader"
	"github.com/opennoty/opennoty/pkg/mongo_util"
	"github.com/opennoty/opennoty/pkg/stdwapper/stdtime"
	"github.com/opennoty/opennoty/pkg/taskqueue"
	"github.com/opennoty/opennoty/pkg/template_engine"
	"github.com/opennoty/opennoty/server/service/notification_service"
	"github.com/opennoty/opennoty/server/service/template_service"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/mail"
	"sync"
	"time"
)

type workflowService struct {
	time stdtime.TimePackage

	templateService     template_service.TemplateService
	notificationService notification_service.NotificationService
	mailSender          mailsender.MailSender

	mutex sync.Mutex

	appCtx         context.Context
	mongoDatabase  *mongo.Database
	leaderElection *mongo_leader.LeaderElection
	brokerClient   taskqueue.Client

	workflowCollection *mongo.Collection
	eventCollection    *mongo.Collection

	eventTriggerBroker taskqueue.Broker

	workLoopTicker   *time.Ticker
	leaderWorkLoopCh chan bool
}

func New(templateService template_service.TemplateService, notificationService notification_service.NotificationService, mailSender mailsender.MailSender) WorkflowService {
	s := &workflowService{
		time:                stdtime.SysTime(),
		templateService:     templateService,
		notificationService: notificationService,
		mailSender:          mailSender,
	}
	return s
}

func (s *workflowService) Start(appCtx context.Context, mongoDatabase *mongo.Database, brokerClient taskqueue.Client, leaderElection *mongo_leader.LeaderElection) error {
	var err error

	s.appCtx = appCtx
	s.brokerClient = brokerClient
	s.leaderElection = leaderElection
	s.mongoDatabase = mongoDatabase
	s.workflowCollection = mongoDatabase.Collection("opennoty.workflows")
	s.eventCollection = mongoDatabase.Collection("opennoty.events")
	s.leaderWorkLoopCh = make(chan bool, 1)

	//s.eventTriggerBroker, err = s.brokerClient.Broker("event.trigger")
	//if err != nil {
	//	return err
	//}

	if err = s.initializeMongo(); err != nil {
		return err
	}

	//eventTriggerCh, err := s.eventTriggerBroker.Consume(appCtx)
	//if err != nil {
	//	return err
	//}
	//go s.eventTriggerEventWorker(s.eventTriggerBroker, eventTriggerCh)

	leaderElection.AddNotify(s.onLeaderChanged)

	go s.leaderWorker()

	return nil
}

func (s *workflowService) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	close(s.leaderWorkLoopCh)
	s.leaderWorkLoopCh = nil
}

func (s *workflowService) Trigger(params *TriggerParams) (string, error) {
	var workflowDoc WorkflowDocument
	var eventDoc EventDocument
	var eventId string

	eventDoc.EventType = EventTypeTrigger
	eventDoc.NextAfterAt = time.UnixMilli(0)
	eventDoc.Tenant = params.Tenant
	eventDoc.Subscriber = params.Subscriber
	if err := eventDoc.DataFrom(params.Event); err != nil {
		return "", err
	}

	err := mongo_util.WithSession(s.mongoDatabase, context.Background(), func(sc mongo.SessionContext) error {
		if err := s.getWorkflow(sc, params.Name, -1, &workflowDoc); err != nil {
			return err
		}

		eventDoc.WorkflowName = workflowDoc.Name
		eventDoc.WorkflowRevision = workflowDoc.Revision
		eventDoc.FlowState = make([]FlowState, len(workflowDoc.Flow))
		eventDoc.FlowError = make([]*string, len(workflowDoc.Flow))

		result, err := s.eventCollection.InsertOne(context.Background(), &eventDoc)
		if err != nil {
			return err
		}
		eventId = mongo_util.InsertedIDToObjectId(result.InsertedID).Hex()
		return nil
	})
	if err != nil {
		return "", err
	}

	if s.eventTriggerBroker != nil {
		if err = s.eventTriggerBroker.Publish(&eventTriggerEventMessage{}); err != nil {
			log.Warnf("[WorkflowService] %v", errors.Wrap(err, "broker publish failed"))
		}
	}

	return eventId, nil
}

func (s *workflowService) CreateWorkflow(doc *WorkflowDocument) (*WorkflowDocument, error) {
	if !CheckWorkflowName(doc.Name) {
		return nil, errors.New("illegal workflow name")
	}
	err := mongo_util.WithTransaction(s.mongoDatabase, context.Background(), func(sc mongo.SessionContext) error {
		var latest WorkflowDocument
		latest.Revision = 0

		cursor, err := s.workflowCollection.Find(sc, bson.M{
			"name": doc.Name,
		}, findLatestWorkflowOption())
		if err != nil {
			return err
		}
		defer cursor.Close(sc)
		if cursor.Next(sc) {
			if err = cursor.Decode(&latest); err != nil {
				return err
			}
		}

		doc.Revision = latest.Revision + 1
		res, err := s.workflowCollection.InsertOne(sc, doc)
		if err != nil {
			return err
		}
		doc.Id = mongo_util.InsertedIDToObjectId(res.InsertedID)

		return nil
	})
	return doc, err
}

func (s *workflowService) GetWorkflow(name string) (*WorkflowDocument, error) {
	var found bool
	doc := new(WorkflowDocument)

	err := mongo_util.WithSession(s.mongoDatabase, context.Background(), func(sc mongo.SessionContext) error {
		cursor, err := s.workflowCollection.Find(sc, bson.M{
			"name": name,
		}, findLatestWorkflowOption())
		if err != nil {
			return err
		}
		defer cursor.Close(sc)
		if cursor.Next(sc) {
			found = true
			if err = cursor.Decode(doc); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if found {
		return doc, nil
	} else {
		return nil, ErrNotFound
	}
}

func (s *workflowService) initializeMongo() error {
	var err error

	_, err = s.workflowCollection.Indexes().CreateMany(context.Background(), workflowIndexes)
	if err != nil {
		return errors.Wrap(err, "[WorkflowService] workflow index create failed")
	}

	return nil
}

func (s *workflowService) onLeaderChanged(result mongo_leader.ElectionResult) {
	if s.workLoopTicker != nil {
		s.workLoopTicker.Stop()
		s.workLoopTicker = nil
	}

	if result.MyRole == mongo_leader.RoleLeader {
		s.workLoopTicker = time.NewTicker(time.Second)
		go func() {
			for s.appCtx.Err() == nil {
				select {
				case <-s.workLoopTicker.C:
					s.mutex.Lock()
					if s.leaderWorkLoopCh != nil {
						s.leaderWorkLoopCh <- true
					}
					s.mutex.Unlock()
				}
			}
		}()
	}
}

func (s *workflowService) leaderWorker() {
	for s.appCtx.Err() == nil {
		_, ok := <-s.leaderWorkLoopCh
		if !ok {
			break
		}

		now := s.time.Now()
		s.doLeaderWork(now)
	}
}

func (s *workflowService) doLeaderWork(now time.Time) {
	err := mongo_util.WithSession(s.mongoDatabase, context.Background(), func(sc mongo.SessionContext) error {
		cursor, err := s.eventCollection.Find(sc, bson.M{
			"finished": false,
			"nextAfterAt": bson.M{
				"$lte": now,
			},
		})
		if err != nil {
			return err
		}
		for cursor.Next(sc) {
			var eventDoc EventDocument
			var workflowDoc WorkflowDocument

			if err = cursor.Decode(&eventDoc); err != nil {
				return err
			}

			if err = s.getWorkflow(sc, eventDoc.WorkflowName, eventDoc.WorkflowRevision, &workflowDoc); err != nil {
				return err
			}

			s.handleQueuedEvent(sc, &eventDoc, &workflowDoc)
		}
		return cursor.Close(sc)
	})
	if err != nil {
		log.Warnf("leader worker error: %v", err)
	}
}

func (s *workflowService) getWorkflow(ctx context.Context, name string, revision int64, doc *WorkflowDocument) error {
	var cursor *mongo.Cursor
	var err error
	if revision < 0 {
		cursor, err = s.workflowCollection.Find(ctx, bson.M{
			"name": name,
		}, findLatestWorkflowOption())
	} else {
		cursor, err = s.workflowCollection.Find(ctx, bson.M{
			"name":     name,
			"revision": revision,
		}, options.Find().SetLimit(1))
	}
	if err != nil {
		return err
	}
	if !cursor.Next(ctx) {
		return &ErrNotExistsWorkflow{Name: name}
	}
	if doc != nil {
		return cursor.Decode(doc)
	}
	return nil
}

func (s *workflowService) getEvent(ctx context.Context, tenantId string, eventId string, doc *EventDocument) error {
	oid, err := primitive.ObjectIDFromHex(eventId)
	if err != nil {
		return err
	}
	workflowQuery := s.eventCollection.FindOne(ctx, bson.M{
		"tenant.id": tenantId,
		"_id":       oid,
	})
	if err = workflowQuery.Err(); err != nil {
		return err
	}
	if doc != nil {
		if err = workflowQuery.Decode(doc); err != nil {
			return err
		}
	}

	return nil
}

func (s *workflowService) getEventList(ctx context.Context, tenantId string, eventIds []primitive.ObjectID) ([]*EventDocument, error) {
	var docList []*EventDocument
	cursor, err := s.eventCollection.Find(ctx, bson.M{
		"tenant.id": tenantId,
		"_id": bson.M{
			"$in": eventIds,
		},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		doc := new(EventDocument)
		if err = cursor.Decode(doc); err != nil {
			return nil, err
		}
		docList = append(docList, doc)
	}
	return docList, nil
}

func (s *workflowService) setProcessedEventFlow(ctx context.Context, eventDoc *EventDocument, flowIndex int) error {
	result := s.eventCollection.FindOneAndUpdate(
		ctx,
		bson.M{
			"_id": eventDoc.Id,
		},
		bson.M{
			"$set": bson.D{
				{fmt.Sprintf("flowState.%d", flowIndex), FlowStateProcessed},
			},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After))
	if err := result.Err(); err != nil {
		return err
	}
	return result.Decode(eventDoc)
}

func (s *workflowService) eventTriggerEventWorker(broker taskqueue.Broker, ch chan taskqueue.Message) {
	for {
		var event eventTriggerEventMessage
		msg, ok := <-ch
		if !ok {
			break
		}
		if err := msg.Parse(&event); err != nil {
			log.Warnf("invalid format event: %v", msg)
			broker.Nack(msg, false)
		} else {
			//if requeue, err := s.handleEventTriggerEvent(broker, msg, &event); err != nil {
			//	log.Warnf("handleEventTriggerEvent failed: %v", err)
			//	broker.Nack(msg, requeue)
			//} else {
			//	log.Warnf("invalid format event: %v", msg)
			//	broker.Ack(msg)
			//}
		}
	}
}

func (s *workflowService) handleQueuedEvent(sc mongo.SessionContext, eventDoc *EventDocument, workflowDoc *WorkflowDocument) {
	var err error

	now := s.time.Now()

	if len(eventDoc.FlowState) != len(workflowDoc.Flow) {
		err = fmt.Errorf("assert (len(eventDoc.FlowState) == len(workflowDoc.Flow)), eventId: %s, workflow: %s", eventDoc.GetId(), workflowDoc.GetId())
		log.Error(err)
		return
	}
	if len(eventDoc.FlowError) == 0 {
		eventDoc.FlowError = make([]*string, len(workflowDoc.Flow))
	}
	if len(eventDoc.FlowError) != len(workflowDoc.Flow) {
		err = fmt.Errorf("assert (len(eventDoc.FlowError) == len(workflowDoc.Flow)), eventId: %s, workflow: %s", eventDoc.GetId(), workflowDoc.GetId())
		log.Error(err)
		return
	}

flowLoop:
	for flowIndex, flowItem := range workflowDoc.Flow {
		var flowErr error
		flowState := eventDoc.FlowState[flowIndex]
		if flowState == FlowStateProcessed {
			continue
		}

		flowState = FlowStateProcessed

		switch flowItem.Type {
		case api_model.FlowDigest:
			var digestDoc EventDocument
			if err = s.upsertLastDigest(sc, now, eventDoc, workflowDoc, flowIndex, &digestDoc); err != nil {
				flowErr = errors.Wrap(err, "upsertLastDigest")
				flowState = FlowStateError
			}
			break flowLoop
		case api_model.FlowNotification:
			if err = s.createNotification(sc, now, eventDoc, workflowDoc, flowIndex); err != nil {
				flowErr = errors.Wrap(err, "createNotification")
				flowState = FlowStateError
			}
		case api_model.FlowEmail:
			if err = s.sendEmail(sc, now, eventDoc, workflowDoc, flowIndex); err != nil {
				flowErr = errors.Wrap(err, "sendEmail")
				flowState = FlowStateError
			}
		}

		if flowErr != nil {
			var strBuf *string = new(string)
			*strBuf = flowErr.Error()
			log.Error(flowErr)
			eventDoc.FlowError[flowIndex] = strBuf
		}
		eventDoc.FlowState[flowIndex] = flowState
		if err = s.saveEventDoc(sc, eventDoc); err != nil {
			err = errors.Wrap(err, "saveEventDoc")
			log.Error(err)
			return
		}
	}

	eventDoc.Finished = true
	if err = s.saveEventDoc(sc, eventDoc); err != nil {
		err = errors.Wrap(err, "saveEventDoc")
		log.Error(err)
	}
}

func (s *workflowService) upsertLastDigest(ctx context.Context, now time.Time, eventDoc *EventDocument, workflowDoc *WorkflowDocument, workflowIndex int, newDoc *EventDocument) error {
	setOnInsert, err := eventDoc.ToBsonForMeta()
	if err != nil {
		return err
	}

	flowItem := workflowDoc.Flow[workflowIndex]

	var newFlowState []FlowState
	newFlowState = make([]FlowState, len(workflowDoc.Flow))
	for i := 0; i <= workflowIndex; i++ {
		newFlowState[i] = FlowStateProcessed
	}

	setOnInsert["_id"] = primitive.NewObjectID()
	setOnInsert["eventType"] = EventTypeDigest
	setOnInsert["nextAfterAt"] = now.Add(time.Second * time.Duration(flowItem.Digest.EventTime))
	setOnInsert["finished"] = false
	setOnInsert["flowState"] = newFlowState

	result := s.eventCollection.FindOneAndUpdate(
		ctx,
		bson.M{
			"tenant.id":            eventDoc.Tenant.Id,
			"subscriber.accountId": eventDoc.Subscriber.AccountId,
			"eventType":            EventTypeDigest,
			"nextAfterAt": bson.M{
				"$gte": now,
			},
		},
		bson.M{
			"$setOnInsert": setOnInsert,
			"$push": bson.M{
				"digestData.eventIds": eventDoc.Id,
			},
		},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After))
	if err := result.Err(); err != nil {
		return err
	}
	return result.Decode(&newDoc)
}

func (s *workflowService) createNotification(ctx context.Context, now time.Time, eventDoc *EventDocument, workflowDoc *WorkflowDocument, workflowIndex int) error {
	flowItem := workflowDoc.Flow[workflowIndex]

	rootContext, err := s.newRenderContext(ctx, eventDoc)
	if err != nil {
		return err
	}

	subject, err := s.templateService.RenderTemplate(flowItem.SubjectTemplate, rootContext)
	if err != nil {
		return err
	}

	stepRaw, err := bson.Marshal(rootContext.Step)
	if err != nil {
		return err
	}
	var stepBson internal_bson.Step
	if err = bson.Unmarshal(stepRaw, &stepBson); err != nil {
		return err
	}

	err = s.notificationService.CreateNotification(&notification_service.NotificationDocument{
		TenantId:  eventDoc.Tenant.Id,
		AccountId: eventDoc.Subscriber.AccountId,
		Subject:   subject,
		Step:      stepBson,
	})
	return err
}

func (s *workflowService) sendEmail(ctx context.Context, now time.Time, eventDoc *EventDocument, workflowDoc *WorkflowDocument, workflowIndex int) error {
	flowItem := workflowDoc.Flow[workflowIndex]

	rootContext, err := s.newRenderContext(ctx, eventDoc)
	if err != nil {
		return err
	}

	subject, err := s.templateService.RenderTemplate(flowItem.SubjectTemplate, rootContext)
	if err != nil {
		return err
	}

	body, err := s.templateService.RenderTemplate(flowItem.ContentTemplate, rootContext)
	if err != nil {
		return err
	}

	return s.mailSender.SendMail(&mailsender.SendMailParams{
		To: []mail.Address{
			{Name: eventDoc.Subscriber.FullName, Address: eventDoc.Subscriber.Email},
		},
		Subject: subject,
		Html:    body,
	})
}

func (s *workflowService) newRenderContext(ctx context.Context, eventDoc *EventDocument) (*template_engine.RootContext, error) {
	var err error
	isDigest := eventDoc.EventType == EventTypeDigest

	root := s.templateService.NewRootContext()
	root.Subscriber = eventDoc.Subscriber
	root.Tenant = eventDoc.Tenant
	root.Step.Digest = isDigest
	if isDigest {
		childEvents, err := s.getEventList(ctx, eventDoc.Tenant.Id, eventDoc.DigestData.EventIds)
		if err != nil {
			return nil, err
		}
		if len(childEvents) != len(eventDoc.DigestData.EventIds) {
			return nil, errors.New("assert len(childEvents) == len(eventDoc.DigestData.EventIds)")
		}
		root.Step.TotalCount = len(childEvents)
		root.Step.Events = make([]template_engine.EventData, len(childEvents))
		for i, event := range childEvents {
			root.Step.Events[i], err = dataToGoObj(event.Data)
			if err != nil {
				return nil, err
			}
		}
	} else {
		root.Step.TotalCount = 1
		root.Step.Events = make([]template_engine.EventData, 1)
		root.Step.Events[0], err = dataToGoObj(eventDoc.Data)
		if err != nil {
			return nil, err
		}
	}

	return root, nil
}

func dataToGoObj(raw []byte) (template_engine.EventData, error) {
	var output template_engine.EventData
	err := bson.Unmarshal(raw, &output)
	return output, err
}

func (s *workflowService) saveEventDoc(ctx context.Context, eventDoc *EventDocument) error {
	var update mongo_util.MongoUpdateStatement
	docRaw, err := bson.Marshal(eventDoc)
	if err != nil {
		return err
	}
	update.Set = docRaw
	result, err := s.eventCollection.UpdateOne(ctx, bson.M{"_id": eventDoc.Id}, &update)
	if err != nil {
		return err
	}
	_ = result
	return nil
}

func findLatestWorkflowOption() *options.FindOptions {
	return options.Find().SetSort(bson.M{"revision": -1}).SetLimit(1)
}
