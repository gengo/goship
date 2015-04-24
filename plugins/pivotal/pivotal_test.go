package pivotal

import (
	"html/template"
	"testing"

	goship "github.com/gengo/goship/lib"
)

var mockProject = goship.Project{
	PluginColumns: nil,
	Name:          "myapp",
	GitHubURL:     "http://github.com/mycompany/myapp",
	RepoName:      "myapp",
	RepoOwner:     "mycompany",
	Environments: []goship.Environment{
		goship.Environment{LatestGitHubCommit: "abc123", Hosts: []goship.Host{{LatestCommit: "abc456"}}},
	},
}

func TestRenderHeader(t *testing.T) {
	c := StoryColumn{
		pivotal: nil,
		Project: mockProject,
	}
	got, err := c.RenderHeader()
	if err != nil {
		t.Errorf(err.Error())
	}
	want := template.HTML("<th style=\"min-width: 200px;\">Stories</th>")
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
