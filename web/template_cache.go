package web

import (
	"encoding/json"
	"html/template"
	"os"
	"sync"
)

var (
	templateCache sync.Map
	templateFuncs = template.FuncMap{
		"json": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
	}
)

func GetTemplate(key string, files ...string) (*template.Template, error) {
	if os.Getenv("ENV") != "production" {
		return parseTemplate(key, files...)
	}

	if cached, ok := templateCache.Load(key); ok {
		return cached.(*template.Template), nil
	}

	tmpl, err := parseTemplate(key, files...)
	if err != nil {
		return nil, err
	}

	actual, _ := templateCache.LoadOrStore(key, tmpl)
	return actual.(*template.Template), nil
}

func parseTemplate(key string, files ...string) (*template.Template, error) {
	return template.New(key).Funcs(templateFuncs).ParseFS(FS, files...)
}
