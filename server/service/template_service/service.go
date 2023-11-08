package template_service

import (
	"context"
	"github.com/opennoty/opennoty/pkg/error_util"
	"github.com/opennoty/opennoty/pkg/template_engine"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type templateService struct {
	appCtx    context.Context
	i18nStore *template_engine.I18nStore
	engine    *template_engine.Engine

	mongoDatabase  *mongo.Database
	i18nCollection *mongo.Collection

	i18nReLoader I18nReLoader
}

func New() TemplateService {
	s := &templateService{
		engine: template_engine.NewEngine(),
	}
	return s
}

func (s *templateService) Start(appCtx context.Context, mongoDatabase *mongo.Database, i18nReLoader I18nReLoader) error {
	s.appCtx = appCtx
	s.mongoDatabase = mongoDatabase
	s.i18nCollection = mongoDatabase.Collection("opennoty.i18ns")

	s.i18nReLoader = i18nReLoader

	return s.ReloadI18n()
}

func (s *templateService) ReloadI18n() error {
	var errs []error

	if err := s.reloadFromDatabase(); err != nil {
		errs = append(errs, err)
	}

	if s.i18nReLoader != nil {
		if err := s.i18nReLoader(s.i18nStore); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return error_util.Multi(errs...)
	}
	return nil
}

func (s *templateService) NewRootContext() *template_engine.RootContext {
	return template_engine.NewRootContext(s.i18nStore)
}

func (s *templateService) RenderTemplate(template string, root *template_engine.RootContext) (string, error) {
	return s.engine.Render(template, map[string]string{}, root)
}

func (s *templateService) initializeMongo() error {
	var err error

	_, err = s.i18nCollection.Indexes().CreateMany(context.Background(), i18nIndexes)
	if err != nil {
		return errors.Wrap(err, "[TemplateService] i18n index create failed")
	}

	return nil
}

func (s *templateService) reloadFromDatabase() error {
	var doc I18nDocument

	cur, err := s.i18nCollection.Find(s.appCtx, bson.M{})
	if err != nil {
		return err
	}

	defer cur.Close(context.Background())

	localeMap := template_engine.I18nRawMap{}

	for cur.Next(s.appCtx) {
		if err = cur.Decode(&doc); err != nil {
			return err
		}
		localeMap.Locale(doc.Locale)[doc.Name] = doc.Message
	}

	return nil
}
