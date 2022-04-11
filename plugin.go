package ecschedule

import (
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/fujiwara/tfstate-lookup/tfstate"
	pkgErrors "github.com/pkg/errors"
)

type Plugin struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config"`
}

func (p Plugin) Setup(c *Config) error {
	switch strings.ToLower(p.Name) {
	case "tfstate":
		return setupPluginTFState(p, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
}

func setupPluginTFState(p Plugin, c *Config) error {
	var loc string
	if p.Config["path"] != nil {
		path, ok := p.Config["path"].(string)
		if !ok {
			return errors.New("tfstate plugin requires path for tfstate file as a string")
		}
		// TODO: validate path
		// if !filepath.IsAbs(path) {
		// 	path = filepath.Join(c.dir, path)
		// }
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
	funcs, err := FuncMapWithName("tfstate", loc)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}

// FuncMapWithName provides a tamplate.FuncMap. can lockup values from tfstate.
func FuncMapWithName(name string, stateLoc string) (template.FuncMap, error) {
	state, err := tfstate.ReadURL(stateLoc)
	if err != nil {
		return nil, pkgErrors.Wrapf(err, "failed to read tfstate: %s", stateLoc)
	}
	nameFunc := func(addrs string) string {
		if tfstateReg.Match([]byte(addrs)) {
			addrs = tfstateReg.FindString(addrs)
		}
		if strings.Contains(addrs, "'") {
			addrs = strings.ReplaceAll(addrs, "'", "\"")
		}
		attrs, err := state.Lookup(addrs)
		if err != nil {
			panic(fmt.Sprintf("failed to lookup %s in tfstate: %s", addrs, err))
		}
		if attrs.Value == nil {
			panic(fmt.Sprintf("%s is not found in tfstate", addrs))
		}
		return attrs.String()
	}
	return template.FuncMap{
		name: nameFunc,
		name + "f": func(format string, args ...interface{}) string {
			addr := fmt.Sprintf(format, args...)
			return nameFunc(addr)
		},
	}, nil
}
