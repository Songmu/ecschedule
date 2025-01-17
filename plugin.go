package ecschedule

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	"github.com/kayac/ecspresso/ssm"
)

// Plugin the plugin
type Plugin struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config"`
}

func (p Plugin) setup(ctx context.Context, c *Config) error {
	switch strings.ToLower(p.Name) {
	case "tfstate":
		return setupPluginTFState(ctx, p, c)
	case "ssm":
		return setupPluginSSM(ctx, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
}

func setupPluginTFState(ctx context.Context, p Plugin, c *Config) error {
	var loc string
	if p.Config["path"] != nil {
		path, ok := p.Config["path"].(string)
		if !ok {
			return errors.New("tfstate plugin requires path for tfstate file as a string")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(c.dir, path)
		}
		loc = path
	} else if p.Config["url"] != nil {
		u, ok := p.Config["url"].(string)
		if !ok {
			return errors.New("tfstate plugin requires url for tfstate URL as a string")
		}
		loc = u
	} else {
		return errors.New("tfstate plugin requires path or url for tfstate location")
	}
	funcs, err := tfstate.FuncMapWithName(ctx, "tfstate", loc)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}

func setupPluginSSM(ctx context.Context, c *Config) error {
	funcs, err := ssm.FuncMap(ctx, getApp(ctx).AwsConf)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}
