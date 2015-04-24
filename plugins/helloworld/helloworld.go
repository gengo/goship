package helloworld

import (
	"html/template"

	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/plugins/plugin"
)

type HelloWorldPlugin struct {
	Column HelloWorldColumn
}

func init() {
	p := &HelloWorldPlugin{HelloWorldColumn{}}
	plugin.RegisterPlugin(p)
}

type HelloWorldColumn struct{}

func (c HelloWorldColumn) RenderHeader() (template.HTML, error) {
	return template.HTML("<th>Example Column</th>"), nil
}

func (c HelloWorldColumn) RenderDetail() (template.HTML, error) {
	return template.HTML("<td>Hello World!</td>"), nil
}

func (p *HelloWorldPlugin) Apply(g goship.Config) error {
	for i := range g.Projects {
		g.Projects[i].AddPluginColumn(p.Column)
	}
	return nil
}
