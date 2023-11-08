package server

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func (s *Server) initPublicApi(router fiber.Router) {
	router.Use(func(ctx *fiber.Ctx) error {
		clientContext := &ClientContext{}
		ctx.Locals(clientContextKey, clientContext)

		if s.option.PublicApiInterceptor != nil {
			err := s.option.PublicApiInterceptor(ctx, clientContext)
			if err != nil {
				return err
			}
		}

		return ctx.Next()
	})

	router.Get("/ws", websocket.New(s.websocketHandler))
}
