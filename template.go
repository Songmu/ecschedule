// copied from github.com/kayac/go-config

package ecsched

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

var envRepTpl *template.Template

func init() {
	envRepTpl = template.New("conf").Funcs(template.FuncMap{
		"env": func(keys ...string) string {
			v := ""
			for _, k := range keys {
				v = os.Getenv(k)
				if v != "" {
					return v
				}
				v = k
			}
			return v
		},
		"must_env": func(key string) string {
			if v, ok := os.LookupEnv(key); ok {
				return v
			}
			panic(fmt.Sprintf("environment variable %s is not defined", key))
		},
	})
}

func envReplacer(data []byte) ([]byte, error) {
	t, err := envRepTpl.Parse(string(data))
	if err != nil {
		return nil, errors.Wrap(err, "config parse by template failed")
	}
	buf := &bytes.Buffer{}
	if err = t.Execute(buf, nil); err != nil {
		// Go 1.12 text/template catches a panic raised in user-defined function.
		// https://golang.org/doc/go1.12#text/template
		if strings.Index(err.Error(), "must_env: environment variable") != -1 {
			panic(err)
		}
		return nil, errors.Wrap(err, "template attach failed")
	}
	return buf.Bytes(), nil
}
