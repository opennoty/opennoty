package mock_mailsender

import (
	"github.com/opennoty/opennoty/pkg/blocking_queue"
	"github.com/opennoty/opennoty/pkg/health"
	"github.com/opennoty/opennoty/pkg/mailsender"
)

type MockMailSender struct {
	SendMailQueue blocking_queue.Queue[*mailsender.SendMailParams]
}

func NewMockMailSender() *MockMailSender {
	return &MockMailSender{
		SendMailQueue: blocking_queue.New[*mailsender.SendMailParams](),
	}
}

func (m *MockMailSender) Check() health.Health {
	return health.Health{
		Status: health.StatusUp,
	}
}

func (m *MockMailSender) SendMail(params *mailsender.SendMailParams) error {
	m.SendMailQueue.PushBack(params)
	return nil
}
