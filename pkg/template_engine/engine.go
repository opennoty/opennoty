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
	"log"
	"regexp"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

// Engine is an implementation of the Helm rendering implementation for templates.
type Engine struct {
	// If strict is enabled, template rendering will fail if a template references
	// a value that was not passed in.
	Strict bool
	// In LintMode, some 'required' template values may be missing, so don't fail
	LintMode bool
}

func NewEngine() *Engine {
	return &Engine{}
}

type Renderable struct {
	tpl  string
	vals interface{}
}

const warnStartDelim = "HELM_ERR_START"
const warnEndDelim = "HELM_ERR_END"
const recursionMaxNums = 1000

var warnRegex = regexp.MustCompile(warnStartDelim + `((?s).*)` + warnEndDelim)

func warnWrap(warn string) string {
	return warnStartDelim + warn + warnEndDelim
}

// initFunMap creates the Engine's FuncMap and adds context-specific functions.
func (e *Engine) initFunMap(t *template.Template) {
	funcMap := funcMap()

	// Override sprig fail function for linting and wrapping message
	funcMap["fail"] = func(msg string) (string, error) {
		if e.LintMode {
			// Don't fail when linting
			log.Printf("[INFO] Fail: %s", msg)
			return "", nil
		}
		return "", errors.New(warnWrap(msg))
	}

	t.Funcs(funcMap)
}

// Render takes a map of templates/values to render, and a map of
// templates which can be referenced within them.
func (e *Engine) Render(mainTemplate string, referencedTemplates map[string]string, values interface{}) (rendered string, err error) {
	// Basically, what we do here is start with an empty parent template and then
	// build up a list of templates -- one for each file. Once all of the templates
	// have been parsed, we loop through again and execute every template.
	//
	// The idea with this process is to make it possible for more complex templates
	// to share common blocks, but to make the entire thing feel like a file-based
	// template engine.
	defer func() {
		if r := recover(); r != nil {
			err = errors.Errorf("rendering template failed: %v", r)
		}
	}()

	t := template.New("gotpl")
	if e.Strict {
		t.Option("missingkey=error")
	} else {
		// Not that zero will attempt to add default values for types it knows,
		// but will still emit <no value> for others. We mitigate that later.
		t.Option("missingkey=zero")
	}

	e.initFunMap(t)

	if _, err = t.New("main").Parse(mainTemplate); err != nil {
		return "", err
	}

	for name, tpl := range referencedTemplates {
		if _, err := t.New(name).Parse(tpl); err != nil {
			return "", err
		}
	}

	var buf strings.Builder
	if err := t.ExecuteTemplate(&buf, "main", values); err != nil {
		return "", err
	}

	rendered = buf.String()

	return rendered, nil
}
