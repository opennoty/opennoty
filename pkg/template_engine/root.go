package template_engine

import (
	"github.com/opennoty/opennoty/api/api_model"
)

type EventData = map[string]any

type RootContext struct {
	i18nStore *I18nStore

	Subscriber api_model.Subscriber `json:"subscriber"`
	Tenant     api_model.Tenant     `json:"tenant"`
	Step       Step                 `json:"step"`
}

type Step struct {
	Digest     bool        `bson:"digest"`
	Events     []EventData `bson:"events"`
	TotalCount int         `bson:"totalCount"`
}

func NewRootContext(i18nStore *I18nStore) *RootContext {
	return &RootContext{
		i18nStore: i18nStore,
	}
}
