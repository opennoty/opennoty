package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/opennoty/opennoty/api/api_proto"
	"github.com/opennoty/opennoty/pkg/health"
	"github.com/opennoty/opennoty/pkg/mailsender"
	"github.com/opennoty/opennoty/pkg/mongo_leader"
	"github.com/opennoty/opennoty/pkg/taskqueue"
	"github.com/opennoty/opennoty/server/service/notification_service"
	"github.com/opennoty/opennoty/server/service/peer_service"
	"github.com/opennoty/opennoty/server/service/pubsub_service"
	"github.com/opennoty/opennoty/server/service/template_service"
	"github.com/opennoty/opennoty/server/service/workflow_service"
	"github.com/prometheus/client_golang/prometheus"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
)

type Server struct {
	option    *Option
	appCtx    context.Context
	appCancel context.CancelFunc

	healthChecker *health.Checker

	mongoClient   *mongo.Client
	mongoDatabase *mongo.Database
	mailSender    mailsender.MailSender
	brokerClient  taskqueue.Client

	peerService         peer_service.PeerService
	leaderElection      *mongo_leader.LeaderElection
	pubSubService       pubsub_service.PubSubService
	notificationService notification_service.NotificationService
	templateService     template_service.TemplateService
	workflowService     workflow_service.WorkflowService

	websocketRequestHandlers map[api_proto.RequestMethod]WebSocketRequestHandler

	PublicApp  *fiber.App
	PrivateApp *fiber.App
}

func NewServer(option *Option) (*Server, error) {
	if option == nil {
		option = &Option{}
	}

	defaultFiberConfig := fiber.Config{
		EnablePrintRoutes: true,
	}

	option.ensureDefaults()

	publicApp := option.PublicApp
	if publicApp == nil {
		publicApp = fiber.New(defaultFiberConfig)
	}

	privateApp := option.PrivateApp
	if privateApp == nil {
		privateApp = fiber.New(defaultFiberConfig)
	}

	contextPath, err := url.JoinPath(option.ContextPath, "/")
	if err != nil {
		return nil, errors.New("invalid context path: " + err.Error())
	}
	if !strings.HasPrefix(contextPath, "/") {
		contextPath = "/" + contextPath
	}
	apiPrefix, err := url.JoinPath(contextPath, "/api")
	if err != nil {
		return nil, errors.New("invalid context path: " + err.Error())
	}

	peerService, err := peer_service.New()
	if err != nil {
		return nil, err
	}

	pubsubService := pubsub_service.New(peerService)
	templateService := template_service.New()
	notificationService := notification_service.New(pubsubService)

	healthChecker := health.NewChecker()

	mailSender, err := mailsender.New(option.SmtpServer, option.MailFrom, option.SmtpUsername, option.SmtpPassword)
	if err != nil {
		return nil, errors.New("mailsender failed: " + err.Error())
	}
	healthChecker.Register("smtp", false, mailSender)
	healthChecker.SetEnabled("smtp", option.SmtpServer != "")

	appCtx, appCancel := context.WithCancel(option.Context)
	s := &Server{
		option:    option,
		appCtx:    appCtx,
		appCancel: appCancel,

		healthChecker: healthChecker,

		peerService:              peerService,
		leaderElection:           mongo_leader.New(),
		pubSubService:            pubsubService,
		notificationService:      notificationService,
		templateService:          templateService,
		workflowService:          workflow_service.New(templateService, notificationService, mailSender),
		websocketRequestHandlers: map[api_proto.RequestMethod]WebSocketRequestHandler{},

		PublicApp:  publicApp,
		PrivateApp: privateApp,
	}

	mongoUri, err := url.Parse(option.MongoUri)
	if err != nil {
		return nil, errors.New("invalid mongodb uri: " + err.Error())
	}
	dbName := strings.TrimPrefix(mongoUri.Path, "/")

	mongoOptions := options.Client().ApplyURI(option.MongoUri)
	if option.MongoUsername != "" {
		mongoCredential := options.Credential{
			Username: option.MongoUsername,
			Password: option.MongoPassword,
		}
		mongoOptions = mongoOptions.SetAuth(mongoCredential)
	}
	s.mongoClient, err = mongo.Connect(context.TODO(), mongoOptions)
	if err != nil {
		return nil, errors.New("mongodb connect failed: " + err.Error())
	}

	s.mongoDatabase = s.mongoClient.Database(dbName, &options.DatabaseOptions{
		BSONOptions: &options.BSONOptions{
			DefaultDocumentM: true,
		},
	})
	s.healthChecker.Register("mongodb", true, &mongoHealthProvider{
		mongoDatabase: s.mongoDatabase,
	})

	//s.brokerClient, err = initBroker(option)
	//if err != nil {
	//	return nil, err
	//}
	//s.healthChecker.Register("broker", true, s.brokerClient)

	publicApp.Use(logger.New())

	root := publicApp.Group(contextPath)
	api := publicApp.Group(apiPrefix)
	s.initPublicApi(api)

	s.initPrivateApi(privateApp)

	registry := prometheus.NewRegistry()
	prometheus := fiberprometheus.NewWithRegistry(registry, "opennoty", "", "", nil)
	prometheus.RegisterAt(privateApp, option.MetricsPath)
	publicApp.Use(prometheus.Middleware)

	_ = root

	s.websocketRequestHandlers[api_proto.RequestMethod_kRequestTopicSubscribe] = s.websocketSubscribe
	s.websocketRequestHandlers[api_proto.RequestMethod_kRequestTopicUnsubscribe] = s.websocketUnsubscribe
	s.websocketRequestHandlers[api_proto.RequestMethod_kRequestStartNotification] = s.websocketStartNotification
	s.websocketRequestHandlers[api_proto.RequestMethod_kRequestFetchNotifications] = s.websocketFetchNotifications
	s.websocketRequestHandlers[api_proto.RequestMethod_kRequestMarkNotifications] = s.websocketMarkNotifications

	return s, nil
}

func initBroker(option *Option) (taskqueue.Client, error) {
	brokerUrl, err := url.Parse(option.BrokerUri)
	if err != nil {
		return nil, errors.New("invalid broker url: " + err.Error())
	}

	var amqConfig amqp.Config
	var brokerUsername = option.BrokerUsername
	var brokerPassword = option.BrokerPassword
	if brokerUsername == "" && brokerUrl.User != nil {
		brokerUsername = brokerUrl.User.Username()
	}
	if brokerPassword == "" && brokerUrl.User != nil {
		brokerPassword, _ = brokerUrl.User.Password()
	}
	if brokerUsername != "" {
		amqConfig.SASL = []amqp.Authentication{
			&amqp.PlainAuth{Username: brokerUsername, Password: brokerPassword},
		}
	}
	return taskqueue.NewAmqpClient(brokerUrl.String(), amqConfig, option.BrokerRoutingKeyPrefix), nil
}

// Option read-only
func (s *Server) Option() Option {
	return *s.option
}

func (s *Server) Start() error {
	var err error
	if err = s.peerService.Start(s.appCtx, fmt.Sprintf(":%d", s.option.ClusterPort), s.option.PeerNotifyAddress, s.mongoDatabase); err != nil {
		return err
	}
	if err = s.leaderElection.Start(s.mongoDatabase, "opennoty.leaders", "leader", s.peerService.GetPeerId()); err != nil {
		return err
	}
	if err = s.pubSubService.Start(s.appCtx, s.mongoDatabase); err != nil {
		return err
	}
	if err = s.templateService.Start(s.appCtx, s.mongoDatabase, s.option.I18nReLoader); err != nil {
		return err
	}
	if err = s.notificationService.Start(s.appCtx, s.mongoDatabase); err != nil {
		return err
	}
	if err = s.workflowService.Start(s.appCtx, s.mongoDatabase, s.brokerClient, s.leaderElection); err != nil {
		return err
	}
	return nil
}

func (s *Server) FiberServe() error {
	var errorCh = make(chan error, 2)
	var refcnt int32 = 2
	var publicAppActivate int32 = 1
	var privateAppActivate int32 = 1

	publicListener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.option.PublicPort))
	if err != nil {
		return err
	}
	defer publicListener.Close()
	s.option.PublicPort = publicListener.Addr().(*net.TCPAddr).Port

	privateListener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.option.PrivatePort))
	if err != nil {
		return err
	}
	defer privateListener.Close()
	s.option.PrivatePort = privateListener.Addr().(*net.TCPAddr).Port

	go func() {
		err := s.PublicApp.Listener(publicListener)
		atomic.StoreInt32(&publicAppActivate, 0)
		errorCh <- err
		if atomic.AddInt32(&refcnt, -1) == 0 {
			close(errorCh)
		}
	}()

	go func() {
		err := s.PrivateApp.Listener(privateListener)
		atomic.StoreInt32(&privateAppActivate, 0)
		errorCh <- err
		if atomic.AddInt32(&refcnt, -1) == 0 {
			close(errorCh)
		}
	}()

	for i := 0; i < 2; i++ {
		err := <-errorCh
		if err != nil {
			if atomic.LoadInt32(&publicAppActivate) == 1 {
				s.PublicApp.Shutdown()
			}
			if atomic.LoadInt32(&privateAppActivate) == 1 {
				s.PrivateApp.Shutdown()
			}
			return err
		}
	}

	return nil
}

func (s *Server) Stop() {
	s.peerService.Stop()
	s.pubSubService.Stop()

	if s.PublicApp != nil {
		s.PublicApp.Shutdown()
	}
	if s.PrivateApp != nil {
		s.PrivateApp.Shutdown()
	}

	if s.appCancel != nil {
		s.appCancel()
		s.appCancel = nil
	}
}

func (s *Server) PeerService() peer_service.PeerService {
	return s.peerService
}

func (s *Server) PubSubService() pubsub_service.PubSubService {
	return s.pubSubService
}
