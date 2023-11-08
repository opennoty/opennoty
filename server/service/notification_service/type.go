package notification_service

import (
	context "context"
	"github.com/opennoty/opennoty/api/api_proto"
	"go.mongodb.org/mongo-driver/mongo"
)

type NotificationFilter struct {
	ContinueToken string // 마지막 가져온 _id
	Limits        int
}

type NotificationService interface {
	Start(appCtx context.Context, mongoDatabase *mongo.Database) error
	CreateNotification(document *NotificationDocument) error
	GetNotifications(tenantId string, accountId string, filter *NotificationFilter) ([]*NotificationDocument, error)
	MarkNotifications(tenantId string, accountId string, params *api_proto.MarkNotificationsRequest) error
}
