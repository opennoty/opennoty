package server

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/opennoty/opennoty/server/service/template_service"
	"net/mail"
	"os"
	"strconv"
	"strings"
)

type PublicApiInterceptor func(c *fiber.Ctx, clientContext *ClientContext) error
type WebsocketInterceptor func(c *WebSocketConnection) error

type Option struct {
	Context context.Context

	PublicPort  int
	ClusterPort int
	PrivatePort int
	ContextPath string
	MetricsPath string

	MongoUri      string
	MongoUsername string
	MongoPassword string

	PeerNotifyAddress string

	// SmtpServer address:port
	SmtpServer      string
	SmtpUsername    string
	SmtpPassword    string
	MailFrom        string
	MailFromName    string
	MailFromAddress string

	BrokerUri              string
	BrokerUsername         string
	BrokerPassword         string
	BrokerRoutingKeyPrefix string

	PublicApp  *fiber.App
	PrivateApp *fiber.App

	WebsocketInterceptor WebsocketInterceptor
	PublicApiInterceptor PublicApiInterceptor
	I18nReLoader         template_service.I18nReLoader

	LocaleFallback string
}

func (o *Option) ensureDefaults() {
	if o.Context == nil {
		o.Context = context.Background()
	}
	if o.PublicPort < 0 {
		o.PublicPort = 0
	} else if o.PublicPort == 0 {
		value := os.Getenv("OPENNOTY_PUBLIC_PORT")
		if value != "" {
			i, err := strconv.ParseInt(value, 10, 16)
			if err != nil {
				panic(fmt.Errorf("OPENNOTY_HTTP_PORT parse failed: %v", err))
			}
			o.PublicPort = int(i)
		} else {
			o.PublicPort = 3000
		}
	}
	if o.PrivatePort < 0 {
		o.PrivatePort = 0
	} else if o.PrivatePort == 0 {
		value := os.Getenv("OPENNOTY_PRIVATE_PORT")
		if value != "" {
			i, err := strconv.ParseInt(value, 10, 16)
			if err != nil {
				panic(fmt.Errorf("OPENNOTY_HTTP_PORT parse failed: %v", err))
			}
			o.PrivatePort = int(i)
		} else {
			o.PrivatePort = o.PublicPort + 1
		}
	}
	if o.ContextPath == "" {
		o.ContextPath = getEnvOrDefault("OPENNOTY_CONTEXT_PATH", "/noty/")
	}
	if o.MetricsPath == "" {
		o.MetricsPath = getEnvOrDefault("OPENNOTY_METRICS_PATH", "/metrics")
	}
	if o.MongoUri == "" {
		o.MongoUri = os.Getenv("MONGODB_URI")
	}
	if o.MongoUsername == "" {
		o.MongoUsername = os.Getenv("MONGODB_USERNAME")
	}
	if o.MongoPassword == "" {
		o.MongoPassword = os.Getenv("MONGODB_PASSWORD")
	}
	if o.SmtpServer == "" {
		o.SmtpServer = os.Getenv("SMTP_SERVER")
	}
	if o.SmtpUsername == "" {
		o.SmtpUsername = os.Getenv("SMTP_USERNAME")
	}
	if o.SmtpPassword == "" {
		o.SmtpPassword = os.Getenv("SMTP_PASSWORD")
	}
	if o.MailFrom == "" {
		o.MailFrom = os.Getenv("MAIL_FROM")
	}
	if o.MailFromName == "" {
		o.MailFromName = os.Getenv("MAIL_FROM_NAME")
	}
	if o.MailFromAddress == "" {
		o.MailFromAddress = os.Getenv("MAIL_FROM_ADDRESS")
	}
	if o.MailFrom == "" && o.MailFromAddress != "" {
		o.MailFrom = (&mail.Address{
			Address: o.MailFromAddress,
			Name:    o.MailFromName,
		}).String()
	}

	if strings.Index(o.SmtpServer, ":") < 0 {
		o.SmtpServer += ":587"
	}

	if o.BrokerUri == "" {
		o.BrokerUri = os.Getenv("BROKER_URI")
	}
	if o.BrokerUsername == "" {
		o.BrokerUsername = os.Getenv("BROKER_USERNAME")
	}
	if o.BrokerPassword == "" {
		o.BrokerPassword = os.Getenv("BROKER_PASSWORD")
	}
	if o.BrokerRoutingKeyPrefix == "" {
		o.BrokerRoutingKeyPrefix = os.Getenv("BROKER_ROUTING_KEY_PREFIX")
	}
	if o.BrokerRoutingKeyPrefix == "" {
		o.BrokerRoutingKeyPrefix = "opennoty."
	}

	if o.LocaleFallback == "" {
		o.LocaleFallback = os.Getenv("LOCALE_FALLBACK")
	}
	if o.LocaleFallback == "" {
		o.LocaleFallback = "en-US"
	}
}

func getEnvOrDefault(name string, def string) string {
	value := os.Getenv(name)
	if value == "" {
		return def
	}
	return value
}
