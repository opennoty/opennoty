package mailsender

import (
	"crypto/tls"
	"github.com/opennoty/opennoty/pkg/health"
	"net/mail"
	"net/smtp"
	"strings"
)

type MailSender interface {
	health.Provider
	SendMail(params *SendMailParams) error
}

type mailSender struct {
	server     string
	serverName string
	mailFrom   *mail.Address
	username   string
	password   string
}

func New(server string, mailFrom string, username string, password string) (MailSender, error) {
	var err error
	var mailFromAddress *mail.Address

	serverNameTokens := strings.Split(server, ":")
	serverName := serverNameTokens[0]

	if mailFrom != "" {
		if mailFromAddress, err = mail.ParseAddress(mailFrom); err != nil {
			return nil, err
		}
	}

	return &mailSender{
		server:     server,
		mailFrom:   mailFromAddress,
		username:   username,
		password:   password,
		serverName: serverName,
	}, nil
}

func (m *mailSender) dial() (*smtp.Client, error) {
	client, err := smtp.Dial(m.server)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			client.Close()
		}
	}()

	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: m.serverName}
		if err = client.StartTLS(config); err != nil {
			return nil, err
		}
	}

	if err = client.Auth(smtp.PlainAuth("", m.username, m.password, m.serverName)); err != nil {
		return nil, err
	}

	return client, nil
}
