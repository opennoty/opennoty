package template_service

import (
	"context"
	"github.com/opennoty/opennoty/pkg/template_engine"
	"go.mongodb.org/mongo-driver/mongo"
)

type I18nReLoader = func(s *template_engine.I18nStore) error

type TemplateService interface {
	Start(appCtx context.Context, mongoDatabase *mongo.Database, i18nReLoader I18nReLoader) error
	ReloadI18n() error

	//TODO: Render(name string, root *template_engine.RootContext) (string, error)

	RenderTemplate(template string, root *template_engine.RootContext) (string, error)
	NewRootContext() *template_engine.RootContext
}
