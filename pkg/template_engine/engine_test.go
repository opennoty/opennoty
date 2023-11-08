/*
Copyright The Helm Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template_engine

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestRender(t *testing.T) {
	templates := []string{
		"{{.Values.outer | title }} {{.Values.inner | title}}",
		"{{.Values.global.callme | lower }}",
		"{{.noValue}}",
	}

	vals := map[string]interface{}{
		"Values": map[string]interface{}{
			"outer": "spouter",
			"inner": "inn",
			"global": map[string]interface{}{
				"callme": "Ishmael",
			},
		},
	}

	expect := []string{
		"Spouter Inn",
		"ishmael",
		"<no value>",
	}

	engine := NewEngine()
	for index, tpl := range templates {
		out, err := engine.Render(tpl, nil, vals)
		if err != nil {
			t.Errorf("Failed to render template[%d]", index)
		}
		assert.Equal(t, expect[index], out)
	}
}

func TestRenderInternals(t *testing.T) {
	// Test the internals of the rendering tool.

	vals := map[string]any{"Name": "one", "Value": "two"}
	tpls := map[string]string{
		"three": `{{template "two" dict "Value" "three"}}`,
	}
	mainTpl := "Hello {{title .Name}}\nGoodbye {{upper .Value}}"

	out, err := new(Engine).Render(mainTpl, tpls, vals)
	if err != nil {
		t.Fatalf("Failed template rendering: %s", err)
	}

	assert.Equal(t, "Hello One\nGoodbye TWO", out)
}

func TestParseErrors(t *testing.T) {
	_, err := new(Engine).Render("{{ foo }}", nil, nil)
	if err == nil {
		t.Fatalf("Expected failures while rendering: %s", err)
	}
	expected := `template: main:1: function "foo" not defined`
	if err.Error() != expected {
		t.Errorf("Expected '%s', got %q", expected, err.Error())
	}
}
