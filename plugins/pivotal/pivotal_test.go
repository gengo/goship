package pivotal

import (
	"html/template"
	"testing"

	goship "github.com/gengo/goship/lib"
)

func TestRenderDetail(t *testing.T) {
	c := StoryColumn{}
	got, err := c.RenderDetail()
	if err != nil {
		t.Errorf(err.Error())
	}
	want := template.HTML(`<td class="story"></td>`)
	if want != got {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestRenderHeader(t *testing.T) {
	c := StoryColumn{}
	got, err := c.RenderHeader()
	if err != nil {
		t.Errorf(err.Error())
	}
	want := template.HTML(`<th style="min-width: 200px;">Stories</th>`)
	if want != got {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestApply(t *testing.T) {
	p := &PivotalPlugin{}
	config := goship.Config{
		Projects: []goship.Project{
			goship.Project{
				RepoName:  "test_project",
				RepoOwner: "test",
			},
		},
		Pivotal: &goship.PivotalConfiguration{Token: "token", Project: "1100"},
	}
	err := p.Apply(config)
	if err != nil {
		t.Fatalf("Error applying plugin %v", err)
	}
	if len(config.Projects[0].PluginColumns) != 1 {
		t.Fatalf("Failed to add plugin column, PluginColumn len = %d", len(config.Projects[0].PluginColumns))
	}
	pl := config.Projects[0].PluginColumns[0]
	switch pl.(type) {
	case StoryColumn:
		break
	default:
		t.Errorf("Plugin is not correct type, type %T", pl)
	}
}
