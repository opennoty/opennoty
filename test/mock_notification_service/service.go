package mock_notification_service

import (
	"context"
	"github.com/opennoty/opennoty/api/api_proto"
	"github.com/opennoty/opennoty/server/service/notification_service"
	"go.mongodb.org/mongo-driver/mongo"
)

type MockNotificationService struct {
}

func NewMockNotificationService() *MockNotificationService {
	return &MockNotificationService{}
}

func (s *MockNotificationService) Start(appCtx context.Context, mongoDatabase *mongo.Database) error {
	return nil
}

func (s *MockNotificationService) CreateNotification(document *notification_service.NotificationDocument) error {
	return nil
}

func (s *MockNotificationService) GetNotifications(tenantId string, accountId string, filter *notification_service.NotificationFilter) ([]*notification_service.NotificationDocument, error) {
	return []*notification_service.NotificationDocument{}, nil
}

func (s *MockNotificationService) MarkNotifications(tenantId string, accountId string, params *api_proto.MarkNotificationsRequest) error {
	return nil
}
