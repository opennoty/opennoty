package mailsender

import (
	"github.com/jhillyerd/enmime"
	"net/mail"
)

type SendMailParams struct {
	To      []mail.Address
	Cc      []mail.Address
	Bcc     []mail.Address
	Subject string
	Text    string
	Html    string
}

func (m *mailSender) SendMail(params *SendMailParams) error {
	var builder enmime.MailBuilder

	builder = builder.Subject(params.Subject)
	builder = builder.From(m.mailFrom.Name, m.mailFrom.Address)

	for _, to := range params.To {
		builder = builder.To(to.Name, to.Address)
	}

	for _, to := range params.Cc {
		builder = builder.CC(to.Name, to.Address)
	}

	for _, to := range params.Bcc {
		builder = builder.BCC(to.Name, to.Address)
	}

	if params.Text != "" {
		builder = builder.Text([]byte(params.Text))
	}
	if params.Html != "" {
		builder = builder.HTML([]byte(params.Html))
	}

	return builder.Send(m)
}

func (m *mailSender) Send(reversePath string, recipients []string, msg []byte) error {
	c, err := m.dial()
	if err != nil {
		return wrapToSendMailError(err)
	}

	defer c.Close()

	if err = c.Mail(reversePath); err != nil {
		return wrapToSendMailError(err)
	}

	for _, addr := range recipients {
		if err = c.Rcpt(addr); err != nil {
			return wrapToSendMailError(err)
		}
	}

	// Data
	wr, err := c.Data()
	if err != nil {
		return wrapToSendMailError(err)
	}

	_, err = wr.Write(msg)
	if err != nil {
		return wrapToSendMailError(err)
	}

	err = wr.Close()
	if err != nil {
		return wrapToSendMailError(err)
	}

	c.Quit()

	return err
}
