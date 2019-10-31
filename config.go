package ecsched

import (
	"io"
	"io/ioutil"

	"github.com/ghodss/yaml"
)

const defaultRole = "ecsEventsRole"

// BaseConfig baseconfig
type BaseConfig struct {
	Region    string `yaml:"region"`
	Cluster   string `yaml:"cluster"`
	AccountID string `yaml:"-"`
}

// Config config
type Config struct {
	Role        string `yaml:"role,omitempty"`
	*BaseConfig `yaml:",inline"`
	Rules       []*Rule `yaml:"rules"`
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

// LoadConfig loads config
func LoadConfig(r io.Reader, accountID string) (*Config, error) {
	c := Config{}
	bs, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	bs, err = envReplacer(bs)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(bs, &c); err != nil {
		return nil, err
	}
	c.AccountID = accountID
	for _, r := range c.Rules {
		r.mergeBaseConfig(c.BaseConfig, c.Role)
	}
	return &c, nil
}
