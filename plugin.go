package ecspresso

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fujiwara/cfn-lookup/cfn"
	"github.com/fujiwara/tfstate-lookup/tfstate"
)

type ConfigPlugin struct {
	Name   string                 `yaml:"name"`
	Config map[string]interface{} `yaml:"config"`
}

func (p ConfigPlugin) Setup(c *Config) error {
	switch strings.ToLower(p.Name) {
	case "tfstate":
		return setupPluginTFState(p, c)
	case "cloudformation":
		return setupPluginCFn(p, c)
	default:
		return fmt.Errorf("plugin %s is not available", p.Name)
	}
}

func setupPluginTFState(p ConfigPlugin, c *Config) error {
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

	funcMapName := "tfstate"
	if p.Config["prefix"] != nil {
		prefix, ok := p.Config["prefix"].(string)
		if !ok {
			return errors.New("as tfstate plugin options, prefix must be a string")
		}
		funcMapName = fmt.Sprintf("%s_%s", prefix, funcMapName)
	}
	funcs, err := tfstate.FuncMapWithName(funcMapName, loc)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}

func setupPluginCFn(p ConfigPlugin, c *Config) error {
	funcs, err := cfn.FuncMap(c.sess)
	if err != nil {
		return err
	}
	c.templateFuncs = append(c.templateFuncs, funcs)
	return nil
}
