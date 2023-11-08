package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/opennoty/opennoty/api/api_model"
	"github.com/opennoty/opennoty/pkg/health"
	"github.com/opennoty/opennoty/server/service/workflow_service"
)

type PeerItem struct {
	Id            string `json:"id"`
	State         string `json:"state"`
	RemoteAddress string `json:"remoteAddress"`
}

type ErrorBody struct {
	Message string `json:"message"`
}

func (s *Server) initPrivateApi(router fiber.Router) {
	router.Get("/health", s.getHealthHandler)
	router.Get("/peers", s.getPeersHandler)
	router.Get("/reload/i18n", s.postReloadI18n)

	router.Post("/pubsub/publish", s.postPubsubPublish)

	router.Post("/workflow/-/", s.postWorkflowCreate)
	router.Post("/workflow/:name", s.getWorkflow)
	router.Post("/workflow/:name/trigger", s.postWorkflowTrigger)
}

func (s *Server) getHealthHandler(c *fiber.Ctx) error {
	resp := s.healthChecker.Check()
	if resp.Status == health.StatusUp {
		return c.Status(200).JSON(resp)
	} else {
		return c.Status(500).JSON(resp)
	}
}

func (s *Server) getPeersHandler(c *fiber.Ctx) error {
	var responseItems []*PeerItem
	peers := s.peerService.GetPeers()
	for _, p := range peers {
		connection := p.GetConnection()

		item := &PeerItem{
			Id:    p.PeerId(),
			State: p.GetState().String(),
		}
		if connection != nil {
			item.RemoteAddress = connection.RemoteAddr().String()
		}
		responseItems = append(responseItems, item)
	}

	return c.Status(200).JSON(responseItems)
}

func (s *Server) postPubsubPublish(c *fiber.Ctx) error {
	headers := c.GetReqHeaders()
	var tenantId string
	tenantIds := headers["X-Tenant-Id"]
	if len(tenantIds) > 1 {
		return c.Status(400).JSON(&ErrorBody{
			Message: "many x-tenant-id header",
		})
	} else if len(tenantIds) == 1 {
		tenantId = tenantIds[0]
	}
	topic := c.Query("topic")

	if err := s.pubSubService.Publish(tenantId, topic, c.BodyRaw()); err != nil {
		return c.Status(500).JSON(&ErrorBody{
			Message: err.Error(),
		})
	}

	return c.Status(201).Send([]byte{})
}

func (s *Server) postReloadI18n(c *fiber.Ctx) error {
	err := s.templateService.ReloadI18n()
	if err == nil {
		return c.SendStatus(201)
	} else {
		return c.Status(500).JSON(&ErrorBody{
			Message: err.Error(),
		})
	}
}

func (s *Server) postWorkflowCreate(c *fiber.Ctx) error {
	var workflow api_model.Workflow
	if err := c.BodyParser(&workflow); err != nil {
		return err
	}

	doc, err := s.workflowService.CreateWorkflow(&workflow_service.WorkflowDocument{
		Name: workflow.Name,
		Flow: workflow.Flow,
	})
	if err != nil {
		return c.Status(500).JSON(&ErrorBody{
			Message: err.Error(),
		})
	}

	workflow.Revision = doc.Revision

	return c.Status(200).JSON(workflow)
}

func (s *Server) getWorkflow(c *fiber.Ctx) error {
	var workflow api_model.Workflow
	name := c.Params("name")
	doc, err := s.workflowService.GetWorkflow(name)
	if err != nil {
		if err == workflow_service.ErrNotFound {
			c.Status(404)
		}
		return c.JSON(&ErrorBody{
			Message: err.Error(),
		})
	}
	workflow.Name = doc.Name
	workflow.Revision = doc.Revision
	workflow.Flow = doc.Flow
	return c.Status(200).JSON(&workflow)
}

func (s *Server) postWorkflowTrigger(c *fiber.Ctx) error {
	var requestBody api_model.WorkflowTriggerRequest
	name := c.Params("name")
	if err := c.BodyParser(&requestBody); err != nil {
		return err
	}

	eventId, err := s.workflowService.Trigger(&workflow_service.TriggerParams{
		Name:       name,
		Tenant:     requestBody.Tenant,
		Subscriber: requestBody.Subscriber,
		Event:      requestBody.Event,
	})
	if err != nil {
		return c.Status(500).JSON(&ErrorBody{
			Message: err.Error(),
		})
	}

	return c.Status(200).JSON(&api_model.WorkflowTriggerResponse{
		EventId: eventId,
	})
}
