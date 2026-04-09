package web

import (
	_ "embed"
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"os"
	"sync"
)

//go:embed templates/*/*.html dist/*
var embeddedFS embed.FS

//go:embed critical.min.css
var criticalCSS string

var FS fs.FS

var (
	templateCache sync.Map
	templateFuncs = template.FuncMap{
		"json": func(v interface{}) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"dict": func(pairs ...interface{}) map[string]interface{} {
			d := make(map[string]interface{}, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				d[pairs[i].(string)] = pairs[i+1]
			}
			return d
		},
	}
)

func init() {
	if os.Getenv("ENV") == "production" {
		FS = embeddedFS
	} else {
		FS = os.DirFS("web")
	}
}

func GetCriticalCSS() template.CSS {
	return template.CSS(criticalCSS)
}

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
