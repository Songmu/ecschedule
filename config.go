package ecschedule

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
	"github.com/google/go-jsonnet"
	gc "github.com/kayac/go-config"
	"github.com/winebarrel/cronplan"
)

const defaultRole = "ecsEventsRole"

const (
	jsonnetExt = ".jsonnet"
	jsonExt    = ".json"
)

// BaseConfig baseconfig
type BaseConfig struct {
	Region    string `yaml:"region" json:"region"`
	Cluster   string `yaml:"cluster" json:"cluster"`
	AccountID string `yaml:"-" json:"-"`
}

// Config config
type Config struct {
	Role        string `yaml:"role,omitempty" json:"role,omitempty"`
	*BaseConfig `yaml:",inline" json:",inline"`
	Rules       []*Rule   `yaml:"rules" json:"rules"`
	Plugins     []*Plugin `yaml:"plugins,omitempty" json:"plugins,omitempty"`

	templateFuncs []template.FuncMap
	dir           string
}

// GetRuleByName gets rule by name
func (c *Config) GetRuleByName(name string) *Rule {
	for _, r := range c.Rules {
		if r.Name == name {
			return r
		}
	}
	return nil
}

func (c *Config) setupPlugins(ctx context.Context) error {
	for _, p := range c.Plugins {
		if err := p.setup(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) cronValidate() error {
	var errMsgs []string
	for _, r := range c.Rules {
		exp := r.ScheduleExpression
		if !strings.HasPrefix(exp, "cron") {
			continue
		}
		_, err := cronplan.Parse(strings.TrimSuffix(strings.TrimPrefix(exp, "cron("), ")"))
		if err != nil {
			errMsg := err.Error()
			if idx := strings.LastIndex(errMsg, ": "); idx != -1 {
				errMsg = errMsg[idx+2:]
			}
			errMsgs = append(errMsgs, fmt.Sprintf("\trule %q: %s", r.Name, errMsg))
		}
	}
	if len(errMsgs) > 0 {
		return fmt.Errorf("cron validation errors:\n%s", strings.Join(errMsgs, "\n"))
	}
	return nil
}

// LoadConfig loads config
func LoadConfig(ctx context.Context, r io.Reader, accountID string, confPath string) (*Config, error) {
	c := Config{}
	bs, ext, err := readConfigFile(r, confPath)
	if err != nil {
		return nil, err
	}
	bs, err = envReplacer(bs)
	if err != nil {
		return nil, err
	}

	if err := unmarshalConfig(bs, &c, ext); err != nil {
		return nil, err
	}
	if err := c.cronValidate(); err != nil {
		return nil, err
	}
	c.AccountID = accountID
	if err := c.setupPlugins(ctx); err != nil {
		return nil, err
	}
	c.dir = filepath.Dir(confPath)
	loader := gc.New()
	for _, f := range c.templateFuncs {
		loader.Funcs(f)
	}
	// recover tfstate variable
	bs = tfstateRecover(bs)
	// recover ssm variable
	bs = ssmRecover(bs)
	bs, err = loader.ReadWithEnvBytes(bs)
	if err != nil {
		return nil, err
	}
	if err := unmarshalConfig(bs, &c, ext); err != nil {
		return nil, err
	}
	for _, r := range c.Rules {
		r.mergeBaseConfig(c.BaseConfig, c.Role)
	}
	return &c, nil
}

// unmarshalConfig unmarshal json or yaml file
func unmarshalConfig(bs []byte, c *Config, ext string) error {
	if ext == jsonExt {
		return json.Unmarshal(bs, c)
	}

	// as a YAML file if the file type cannot be determined from the extension (e.g. .ecschedule, ecschedule.cfg)
	return yaml.Unmarshal(bs, c)
}

func readConfigFile(r io.Reader, confPath string) ([]byte, string, error) {
	ext := filepath.Ext(confPath)
	if ext == jsonnetExt {
		vm := jsonnet.MakeVM()
		bs, err := vm.EvaluateFile(confPath)
		if err != nil {
			return nil, ext, fmt.Errorf("failed to evaluate jsonnet file: %w", err)
		}
		return []byte(bs), jsonExt, err
	}
	bs, err := ioutil.ReadAll(r)
	return bs, ext, err
}
