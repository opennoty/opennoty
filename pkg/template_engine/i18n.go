package template_engine

import (
	"bytes"
	"errors"
	"html/template"
	"strings"
	"sync"
)

var ErrNotExists = errors.New("not exists")

type I18nStore struct {
	lock           sync.RWMutex
	localeMap      map[string]map[string]*template.Template
	fallbackLocale string
}

func NewI18nStore() *I18nStore {
	return &I18nStore{
		localeMap: map[string]map[string]*template.Template{},
	}
}

func (s *I18nStore) SetFallbackLocale(locale string) {
	s.fallbackLocale = strings.ToLower(locale)
}

func (s *I18nStore) LoadMessages(locale string, data map[string]string) error {
	locale = strings.ToLower(locale)

	messageMap := map[string]*template.Template{}
	for key, value := range data {
		tmpl := template.New(key)
		_, err := tmpl.Parse(value)
		if err != nil {
			return err
		}
		messageMap[key] = tmpl
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	s.localeMap[locale] = messageMap

	return nil
}

func (s *I18nStore) ResolveMessage(locale string, name string, arguments map[string]any) (string, error) {
	locale = strings.ToLower(locale)
	name = strings.ToLower(name)

	s.lock.RLock()
	defer s.lock.RUnlock()

	fallbackMap, hasFallback := s.localeMap[s.fallbackLocale]

	messageMap, ok := s.localeMap[locale]
	if !ok {
		messageMap, ok = fallbackMap, hasFallback
	}
	if !ok {
		return "", ErrNotExists
	}

	messageTemplate, ok := messageMap[name]
	if !ok && hasFallback {
		messageTemplate, ok = fallbackMap[name]
	}
	if !ok {
		return "", ErrNotExists
	}
	var buffer bytes.Buffer
	if err := messageTemplate.Execute(&buffer, arguments); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

type I18nRawMap map[string]map[string]string

func (m I18nRawMap) Locale(locale string) map[string]string {
	locale = strings.ToLower(locale)
	messageMap, ok := m[locale]
	if !ok {
		messageMap = map[string]string{}
		m[locale] = messageMap
	}
	return messageMap
}
