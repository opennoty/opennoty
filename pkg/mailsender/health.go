package mailsender

import "github.com/opennoty/opennoty/pkg/health"

func (m *mailSender) Check() health.Health {
	resp := health.Health{
		Status: health.StatusUp,
	}

	client, err := m.dial()
	if err != nil {
		resp.Status = health.StatusDown
		resp.Reason = err.Error()
		return resp
	}

	client.Quit()
	client.Close()

	return resp
}
