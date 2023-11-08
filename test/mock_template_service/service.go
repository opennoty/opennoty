package mock_template_service

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/opennoty/opennoty/pkg/template_engine"
	"github.com/opennoty/opennoty/server/service/template_service"
	"go.mongodb.org/mongo-driver/mongo"
	"strings"
)

//go:embed sample_i18n.json
var sampleI18nJson []byte

type SampleTemplatesI18n map[string]map[string]string

type MockTemplateService struct {
	i18nStore *template_engine.I18nStore
	engine    *template_engine.Engine

	templates map[string]string
}

func NewMockTemplateService() *MockTemplateService {
	var sampleI18n SampleTemplatesI18n
	if err := json.Unmarshal(sampleI18nJson, &sampleI18n); err != nil {
		panic(err)
	}

	m := &MockTemplateService{
		i18nStore: template_engine.NewI18nStore(),
		engine:    template_engine.NewEngine(),
	}

	for locale, messages := range sampleI18n {
		m.i18nStore.LoadMessages(locale, messages)
	}

	return m
}

func (m *MockTemplateService) Start(appCtx context.Context, mongoDatabase *mongo.Database, i18nReLoader template_service.I18nReLoader) error {
	return nil
}

func (m *MockTemplateService) ReloadI18n() error {
	return nil
}

func (m *MockTemplateService) RenderTemplate(template string, root *template_engine.RootContext) (string, error) {
	return m.engine.Render(template, m.templates, root)
}

func (m *MockTemplateService) Render(name string, root *template_engine.RootContext) (string, error) {
	var templates = map[string]string{}
	name = strings.ToLower(name)

	for k, v := range m.templates {
		templates[k] = v
	}

	mainTpl, ok := templates[name]
	if !ok {
		return "", errors.New("no template: " + name)
	}
	delete(templates, name)

	return m.engine.Render(mainTpl, templates, root)
}

func (m *MockTemplateService) NewRootContext() *template_engine.RootContext {
	return template_engine.NewRootContext(m.i18nStore)
}
