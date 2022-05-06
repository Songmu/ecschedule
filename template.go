// copied from github.com/kayac/go-config

package ecschedule

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

var envRepTpl *template.Template
var tfstateRepRegex = regexp.MustCompile("ecschedule::tfstate::<`(.*)`>")
var tfstatefRepRegex = regexp.MustCompile("ecschedule::tfstatef::<`(.*)`>")

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
			return fmt.Sprintf("ecschedule::<%s>", key)
		},
		"tfstate": func(key string) string {
			return fmt.Sprintf("ecschedule::tfstate::<`%s`>", key)
		},
		"tfstatef": func(key string, args ...string) string {
			return fmt.Sprintf("ecschedule::tfstatef::<`%s` `%s`>", key, strings.Join(args, "` `"))
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
		return nil, errors.Wrap(err, "template attach failed")
	}
	return buf.Bytes(), nil
}

func tfstateRecover(data []byte) []byte {
	s := tfstateRepRegex.ReplaceAllString(string(data), "{{ tfstate `$1` }}")
	s = tfstatefRepRegex.ReplaceAllString(s, "{{ tfstatef `$1` }}")
	return []byte(s)
}
