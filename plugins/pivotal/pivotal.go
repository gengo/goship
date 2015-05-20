package pivotal

import (
	"html/template"

	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/plugins/plugin"
)

type PivotalPlugin struct {
	Column StoryColumn
}

func init() {
	p := &PivotalPlugin{}
	plugin.RegisterPlugin(p)
}

type StoryColumn struct{}

func (c StoryColumn) RenderHeader() (template.HTML, error) {
	return template.HTML(`<th style="min-width: 200px;">Stories</th>`), nil
}

func (c StoryColumn) RenderDetail() (template.HTML, error) {
	return template.HTML(`<td class="story"></td>`), nil
}

func (p *PivotalPlugin) Apply(g goship.Config) error {
	for i := range g.Projects {
		g.Projects[i].AddPluginColumn(StoryColumn{})
	}
	return nil
}
