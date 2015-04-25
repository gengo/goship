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

var BootstrapLabel = map[string]string{
	"_base":     "<span class=\"label label-%s\">%s</span>",
	"planned":   "default", // unstarted is `planned` in API response
	"started":   "info",
	"finished":  "primary",
	"delivered": "warning",
	"accepted":  "success",
	"rejected":  "danger",
}

var GetPivotalIDFromGithubCommits = goship.GetPivotalIDFromCommits

const (
	pivotalURL      = "https://www.pivotaltracker.com"
	pivotalAPIURL   = pivotalURL + "/services/v5"
	pivotalStoryURL = pivotalURL + "/story/show"
)

type PivotalPlugin struct {
	Column StoryColumn
	Client PivotalClientInterface
}

func init() {
	p := &PivotalPlugin{}
	plugin.RegisterPlugin(p)
}

type PivotalClientInterface interface {
	GetStoryStatus(string) (string, error)
}

type PivotalClient struct {
	Token string
}

func (pc PivotalClient) GetStoryStatus(pivotalID string) (string, error) {
	pivotalStoryAPIURL := pivotalAPIURL + "/stories/%s"
	url := fmt.Sprintf(pivotalStoryAPIURL, pivotalID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-TrackerToken", pc.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var psr = new(PivotalStoryResponse)
	json.NewDecoder(resp.Body).Decode(&psr)
	return psr.Status, nil
}

type StoryColumn struct {
	Project       goship.Project
	PivotalClient PivotalClientInterface
}

// partial representation of the full json response from Pivotal
type PivotalStoryResponse struct {
	Status string `json:"current_state"`
}

func (c StoryColumn) RenderHeader() (template.HTML, error) {
	return template.HTML("<th style=\"min-width: 200px;\">Stories</th>"), nil
}

func (c StoryColumn) RenderDetail() (template.HTML, error) {
	var content = ""
	var infoTmpl = "<a href=\"%s/%s\" target=\"_blank\">%s</a> %s<br/>"
	var owner = c.Project.RepoOwner
	var repo = c.Project.RepoName
	//var latestCommit = ""
	//var currentCommit = ""
	// TODO: need to know environment context at every renderDetail level

	//pivotalIDs, err := GetPivotalIDFromGithubCommits(owner, repo, latestCommit, currentCommit)
	pivotalIDs, err := GetPivotalIDFromGithubCommits(owner, repo, "095168b87e702173ba7265e4287f4f8f96f1bb18", "4833a8a4e41b39099c5c7e08f78046bd842de5e7")
	if err != nil {
		log.Printf("Failed to obtain pivotal IDs from Github commits. err: %s", err)
		return template.HTML("<td></td>"), nil
	}
	for _, id := range pivotalIDs {
		// NOTE: could we make use of goroutines and channels to make this more efficient?
		status, err := c.PivotalClient.GetStoryStatus(id)
		if err != nil {
			log.Printf("Failed to obtain story status for Pivotal ticket #%s. err: %s", id, err)
		}
		label := fmt.Sprintf(BootstrapLabel["_base"], BootstrapLabel[status], status) // make bootstrap label based on status
		info := fmt.Sprintf(infoTmpl, pivotalStoryURL, id, id, label)
		content += info // add ticket info into content template
	}
	return template.HTML(fmt.Sprintf("<td>%s</td>", content)), nil
}

func (p *PivotalPlugin) Apply(g goship.Config) error {
	p.Client = PivotalClient{Token: g.Pivotal.Token}
	for i := range g.Projects {
		g.Projects[i].AddPluginColumn(StoryColumn{
			Project:       g.Projects[i],
			PivotalClient: p.Client,
		})
	}
	return nil
}
