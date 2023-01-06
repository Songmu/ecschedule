package ecschedule

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"text/template"

	"github.com/goccy/go-yaml"
	gc "github.com/kayac/go-config"
)

const defaultRole = "ecsEventsRole"

const (
	jsonExt = ".json"
	ymlExt  = ".yml"
	yamlExt = ".yaml"
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

// LoadConfig loads config
func LoadConfig(ctx context.Context, r io.Reader, accountID string, confPath string) (*Config, error) {
	c := Config{}
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	bs, err = envReplacer(bs)
	if err != nil {
		return nil, err
	}
	if err := unmarshalConfig(bs, &c, confPath); err != nil {
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
	bs, err = loader.ReadWithEnvBytes(bs)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(bs, &c); err != nil {
		return nil, err
	}
	for _, r := range c.Rules {
		r.mergeBaseConfig(c.BaseConfig, c.Role)
	}
	return &c, nil
}

// unmarshalConfig unmarshal json or yaml file
func unmarshalConfig(bs []byte, c *Config, filePath string) error {
	switch filepath.Ext(filePath) {
	case jsonExt:
		return json.Unmarshal(bs, c)
	case yamlExt, ymlExt:
		return yaml.Unmarshal(bs, c)
	}
	return fmt.Errorf("not supported file type")
}
