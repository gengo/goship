package pivotal

import (
	"html/template"

	"github.com/gengo/goship/lib/config"
	"github.com/gengo/goship/plugins/plugin"
)

type PivotalPlugin struct {
	Column StoryColumn
}

func init() {
	var p PivotalPlugin
	plugin.RegisterPlugin(p)
}

type StoryColumn struct{}

func (c StoryColumn) RenderHeader() (template.HTML, error) {
	return template.HTML(`<th style="min-width: 200px;">Stories</th>`), nil
}

func (c StoryColumn) RenderDetail() (template.HTML, error) {
	return template.HTML(`<td class="story"></td>`), nil
}

func (p PivotalPlugin) Apply(proj config.Project) ([]plugin.Column, error) {
	return []plugin.Column{p.Column}, nil
}
