package template_engine

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func testI18nStore() *I18nStore {
	i18nStore := NewI18nStore()
	i18nStore.SetFallbackLocale("ko-kr")
	i18nStore.LoadMessages("ko-kr", map[string]string{
		"hello": `{{ .name }} 님 안녕하세요`,
	})
	i18nStore.LoadMessages("en-us", map[string]string{
		"hello": `Hello, {{ .name }}`,
	})
	return i18nStore
}

func TestI18nResolve(t *testing.T) {
	i18n := NewI18nStore()

	if err := i18n.LoadMessages("en-us", map[string]string{
		"welcome": "Welcome, {{ .Name }}.",
	}); err != nil {
		t.Error(err)
		return
	}
	if err := i18n.LoadMessages("ko-kr", map[string]string{
		"welcome": "환영합니다 {{ .Name }} 님.",
	}); err != nil {
		t.Error(err)
		return
	}

	result, err := i18n.ResolveMessage("ko-kr", "welcome", map[string]interface{}{
		"Name": "gil-dong hong",
	})
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, "환영합니다 gil-dong hong 님.", result)

	result, err = i18n.ResolveMessage("en-us", "welcome", map[string]interface{}{
		"Name": "gil-dong hong",
	})
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, "Welcome, gil-dong hong.", result)
}
