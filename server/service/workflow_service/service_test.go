package workflow_service

import (
	"context"
	_ "embed"
	"errors"
	"github.com/opennoty/opennoty/api/api_model"
	"github.com/opennoty/opennoty/test/mock_mailsender"
	"github.com/opennoty/opennoty/test/mock_notification_service"
	"github.com/opennoty/opennoty/test/mock_template_service"
	"github.com/opennoty/opennoty/test/mock_time"
	"github.com/opennoty/opennoty/test/testhelper"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

//go:embed testdata/sample_workflows.json
var sampleWorkflowsJson []byte

type MockServices struct {
	Ctx                 context.Context
	MailSender          *mock_mailsender.MockMailSender
	TemplateService     *mock_template_service.MockTemplateService
	NotificationService *mock_notification_service.MockNotificationService
	WorkflowService     *workflowService
	MockTime            *mock_time.MockTimePackage
}

func createMockService() *MockServices {
	testEnv := testhelper.GetTestEnv()

	m := &MockServices{
		Ctx:                 context.Background(),
		MailSender:          mock_mailsender.NewMockMailSender(),
		TemplateService:     mock_template_service.NewMockTemplateService(),
		NotificationService: mock_notification_service.NewMockNotificationService(),
		MockTime:            mock_time.NewMockTime(),
	}
	mongoDatabase := testEnv.GetMongoDatabase()
	m.WorkflowService = &workflowService{
		time:                m.MockTime,
		templateService:     m.TemplateService,
		notificationService: m.NotificationService,
		mailSender:          m.MailSender,
		appCtx:              m.Ctx,
		leaderWorkLoopCh:    make(chan bool, 1),
		mongoDatabase:       mongoDatabase,
		eventCollection:     mongoDatabase.Collection("opennoty.events"),
		workflowCollection:  mongoDatabase.Collection("opennoty.workflows"),
	}

	return m
}

func TestDigest(t *testing.T) {
	m := createMockService()

	m.MockTime.SetNow(time.Now())

	eventId, err := m.WorkflowService.Trigger(&TriggerParams{
		Name: "user-activity",
		Tenant: api_model.Tenant{
			Id:   "00000000-0001-4000-0000-000000000001",
			Name: "test company",
		},
		Subscriber: api_model.Subscriber{
			Email:    "test@example.com",
			FullName: "Tester Lee",
			Locale:   "en-us",
		},
		Event: map[string]any{
			"TEST": "1234",
		},
	})
	if err != nil {
		t.Errorf("Trigger Failed: %v", err)
		return
	}
	assert.NotEmpty(t, eventId)

	m.WorkflowService.doLeaderWork(m.MockTime.Now())
	assert.Equal(t, 0, m.MailSender.SendMailQueue.Size())

	m.MockTime.SetNow(m.MockTime.Now().Add(time.Second * 6))

	m.WorkflowService.doLeaderWork(m.MockTime.Now())
	assert.Equal(t, 1, m.MailSender.SendMailQueue.Size())

	mailParams, ok := m.MailSender.SendMailQueue.Pop()
	if !ok {
		panic(errors.New("no data"))
	}
	assert.Equal(t, "TestMail: Today, Tester Lee's activities: 1", mailParams.Subject)

	// something check
}

func TestMain(m *testing.M) {
	testhelper.SetTimeout(time.Second * 10)
	testhelper.InsertDocumentsFromJson[WorkflowDocument]("opennoty.workflows", sampleWorkflowsJson)
	testhelper.TestMain(m)
}
