package pivotal

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/plugins/plugin"
)

var bootstrapLabel = map[string]string{
	"_base":     "<span class=\"label label-%s\">%s</span>",
	"planned":   "default", // unstarted is `planned` in API response
	"started":   "info",
	"finished":  "primary",
	"delivered": "warning",
	"accepted":  "success",
	"rejected":  "danger",
}

const (
	pivotalURL      = "https://www.pivotaltracker.com"
	pivotalAPIURL   = pivotalURL + "/services/v5"
	pivotalStoryURL = pivotalURL + "/story/show"
)

type PivotalPlugin struct {
	Column StoryColumn
}

func init() {
	p := &PivotalPlugin{StoryColumn{}}

	plugin.RegisterPlugin(p)
}

type StoryColumn struct {
	pivotal *goship.PivotalConfiguration
	Project goship.Project
}

// partial representation of the full json response from Pivotal
type PivotalStoryResponse struct {
	Status string `json:"current_state"`
}

func (c StoryColumn) GetPivotalStoryStatus(pivotalID string) string {
	pivotalStoryAPIURL := pivotalAPIURL + "/stories/%s"
	url := fmt.Sprintf(pivotalStoryAPIURL, pivotalID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Failed to make requet to %s. err: %s", url, err)
		return ""
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-TrackerToken", c.pivotal.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("Fail to get response from %s. err: %s", url, err)
		return ""
	}
	var psr = new(PivotalStoryResponse)
	json.NewDecoder(resp.Body).Decode(&psr)
	return psr.Status
}

func (c StoryColumn) RenderHeader() (template.HTML, error) {
	return template.HTML("<th style=\"min-width: 200px;\">Stories</th>"), nil
}

func (c StoryColumn) RenderDetail() (template.HTML, error) {
	var content = ""
	var infoTmpl = "<a href=\"%s/%s\" target=\"_blank\">%s</a> %s<br/>"
	var owner = c.Project.RepoOwner
	var repo = c.Project.RepoName
	var latestCommit = ""
	var currentCommit = ""
	for _, env := range c.Project.Environments {
		for _, host := range env.Hosts {
			if host.GitHubDiffURL != "" {
				latestCommit = env.LatestGitHubCommit
				currentCommit = host.LatestCommit
				break
			}
		}
		pivotalIDs, err := goship.GetPivotalIDFromCommits(owner, repo, latestCommit, currentCommit)
		//pivotalIDs, err := goship.GetPivotalIDFromCommits(owner, repo, "095168b87e702173ba7265e4287f4f8f96f1bb18", "4833a8a4e41b39099c5c7e08f78046bd842de5e7")
		if err != nil {
			log.Printf("Failed to obtain pivotal IDs from Github commits. err: %s", err)
			return template.HTML("<td></td>"), nil
		}
		for _, id := range pivotalIDs {
			status := c.GetPivotalStoryStatus(id)
			label := fmt.Sprintf(bootstrapLabel["_base"], bootstrapLabel[status], status) // make bootstrap label based on status
			info := fmt.Sprintf(infoTmpl, pivotalStoryURL, id, id, label)
			content += info // add ticket info into content template
		}
	}
	return template.HTML(fmt.Sprintf("<td>%s</td>", content)), nil
}

func (p *PivotalPlugin) Apply(g goship.Config) error {
	for i := range g.Projects {
		g.Projects[i].AddPluginColumn(StoryColumn{
			pivotal: g.Pivotal,
			Project: g.Projects[i],
		})
	}
	return nil
}
