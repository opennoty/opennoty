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
	"github.com/opennoty/opennoty/api/api_model"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestFuncs(t *testing.T) {
	//TODO write tests for failure cases
	tests := []struct {
		tpl, expect string
		vars        interface{}
	}{{
		tpl:    `{{ toJson . }}`,
		expect: `{"foo":"bar"}`,
		vars:   map[string]interface{}{"foo": "bar"},
	}, {
		tpl:    `{{ fromJson .}}`,
		expect: `map[hello:world]`,
		vars:   `{"hello":"world"}`,
	}, {
		tpl:    `{{ fromJson . }}`,
		expect: `map[Error:json: cannot unmarshal array into Go value of type map[string]interface {}]`,
		vars:   `["one", "two"]`,
	}, {
		tpl:    `{{ fromJsonArray . }}`,
		expect: `[one 2 map[name:helm]]`,
		vars:   `["one", 2, { "name": "helm" }]`,
	}, {
		tpl:    `{{ fromJsonArray . }}`,
		expect: `[json: cannot unmarshal object into Go value of type []interface {}]`,
		vars:   `{"hello": "world"}`,
	}}

	for _, tt := range tests {
		var b strings.Builder
		err := template.Must(template.New("test").Funcs(funcMap()).Parse(tt.tpl)).Execute(&b, tt.vars)
		assert.NoError(t, err)
		assert.Equal(t, tt.expect, b.String(), tt.tpl)
	}
}

//// This test to check a function provided by sprig is due to a change in a
//// dependency of sprig. mergo in v0.3.9 changed the way it merges and only does
//// public fields (i.e. those starting with a capital letter). This test, from
//// sprig, fails in the new version. This is a behavior change for mergo that
//// impacts sprig and Helm users. This test will help us to not update to a
//// version of mergo (even accidentally) that causes a breaking change. See
//// sprig changelog and notes for more details.
//// Note, Go modules assume semver is never broken. So, there is no way to tell
//// the tooling to not update to a minor or patch version. `go install` could
//// be used to accidentally update mergo. This test and message should catch
//// the problem and explain why it's happening.
//func TestMerge(t *testing.T) {
//	dict := map[string]interface{}{
//		"src2": map[string]interface{}{
//			"h": 10,
//			"i": "i",
//			"j": "j",
//		},
//		"src1": map[string]interface{}{
//			"a": 1,
//			"b": 2,
//			"d": map[string]interface{}{
//				"e": "four",
//			},
//			"g": []int{6, 7},
//			"i": "aye",
//			"j": "jay",
//			"k": map[string]interface{}{
//				"l": false,
//			},
//		},
//		"dst": map[string]interface{}{
//			"a": "one",
//			"c": 3,
//			"d": map[string]interface{}{
//				"f": 5,
//			},
//			"g": []int{8, 9},
//			"i": "eye",
//			"k": map[string]interface{}{
//				"l": true,
//			},
//		},
//	}
//	tpl := `{{merge .dst .src1 .src2}}`
//	var b strings.Builder
//	err := template.Must(template.NewI18nStore("test").Funcs(funcMap()).Parse(tpl)).Execute(&b, dict)
//	assert.NoError(t, err)
//
//	expected := map[string]interface{}{
//		"a": "one", // key overridden
//		"b": 2,     // merged from src1
//		"c": 3,     // merged from dst
//		"d": map[string]interface{}{ // deep merge
//			"e": "four",
//			"f": 5,
//		},
//		"g": []int{8, 9}, // overridden - arrays are not merged
//		"h": 10,          // merged from src2
//		"i": "eye",       // overridden twice
//		"j": "jay",       // overridden and merged
//		"k": map[string]interface{}{
//			"l": true, // overridden
//		},
//	}
//	assert.Equal(t, expected, dict["dst"])
//}

func TestI18nFunc(t *testing.T) {
	engine := NewEngine()

	rootCtx := &RootContext{
		i18nStore: testI18nStore(),
		Subscriber: api_model.Subscriber{
			Locale: "ko-kr",
		},
	}

	out, err := engine.Render(`test {{ i18n . "hello" (dict "name" "HongGilDong") }}.`, nil, &rootCtx)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, "test HongGilDong 님 안녕하세요.", out)

	rootCtx.Subscriber.Locale = "en-us"
	out, err = engine.Render(`test {{ i18n . "hello" (dict "name" "HongGilDong") }}.`, nil, &rootCtx)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, "test Hello, HongGilDong.", out)

	rootCtx.Subscriber.Locale = "unknown"
	out, err = engine.Render(`test {{ i18n . "hello" (dict "name" "HongGilDong") }}.`, nil, &rootCtx)
	if err != nil {
		t.Error(err)
		return
	}
	assert.Equal(t, "test HongGilDong 님 안녕하세요.", out)
}
