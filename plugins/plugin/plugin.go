package plugin

import (
	"github.com/gengo/goship/lib/config"
)

var Plugins []Plugin

type Plugin interface {
	Apply(config.Config) error
}

func RegisterPlugin(p Plugin) {
	Plugins = append(Plugins, p)
}
