package plugins

import goship "github.com/gengo/goship/lib"

var Plugins []Plugin

type Plugin interface {
	Apply(goship.Config) error
}

func RegisterPlugin(p Plugin) {
	Plugins = append(Plugins, p)
}
