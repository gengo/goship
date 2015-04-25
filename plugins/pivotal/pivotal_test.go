package pivotal

import (
	"fmt"
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

type MockPivotalClient struct{}

func (pc MockPivotalClient) GetStoryStatus(pivotalID string) (string, error) {
	return "delivered", nil
}

func TestRenderDetail(t *testing.T) {
	c := StoryColumn{
		Project:       mockProject,
		PivotalClient: MockPivotalClient{},
	}
	// patch
	var mockIDs = []string{"1234", "2345", "3456"}
	// patching the response from goship.GetPivotalIDFromCommits
	GetPivotalIDFromGithubCommits = func(_, _, _, _ string) ([]string, error) {
		return mockIDs, nil
	}

	got, err := c.RenderDetail()
	if err != nil {
		t.Errorf(err.Error())
	}
	var content = ""
	for _, id := range mockIDs {
		var infoTmpl = "<a href=\"%s/%s\" target=\"_blank\">%s</a> %s<br/>"
		label := fmt.Sprintf(BootstrapLabel["_base"], BootstrapLabel["delivered"], "delivered")
		info := fmt.Sprintf(infoTmpl, pivotalStoryURL, id, id, label)
		content += info
	}
	want := template.HTML(fmt.Sprintf("<td>%s</td>", content))
	if want != got {
		t.Errorf("Want %#v, got %#v", want, got)
	}
}

func TestRenderHeader(t *testing.T) {
	c := StoryColumn{
		Project:       mockProject,
		PivotalClient: nil,
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
