package helloworld

import (
	"html/template"

	"github.com/gengo/goship/lib/config"
	"github.com/gengo/goship/plugins/plugin"
)

type HelloWorldPlugin struct {
	Column HelloWorldColumn
}

func init() {
	var p HelloWorldPlugin
	plugin.RegisterPlugin(p)
}

type HelloWorldColumn struct{}

func (c HelloWorldColumn) RenderHeader() (template.HTML, error) {
	return template.HTML("<th>Example Column</th>"), nil
}

func (c HelloWorldColumn) RenderDetail() (template.HTML, error) {
	return template.HTML("<td>Hello World!</td>"), nil
}

func (p HelloWorldPlugin) Apply(proj config.Project) ([]plugin.Column, error) {
	return []plugin.Column{p.Column}, nil
}
