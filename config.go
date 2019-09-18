package ecsched

import (
	"io"
	"io/ioutil"

	"github.com/ghodss/yaml"
)

const defaultRole = "ecsEventsRole"

type BaseConfig struct {
	Region    string `json:"region"`
	Cluster   string `json:"cluster"`
	AccountID string `json:"-"`
}

type Config struct {
	Role string `json:"role,omitempty"`
	*BaseConfig
	Rules []*Rule `json:"rules"`
}

func (c *Config) GetRuleByName(name string) *Rule {
	for _, r := range c.Rules {
		if r.Name == name {
			return r
		}
	}
	return nil
}

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
