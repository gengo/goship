package pivotal

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"regexp"

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
	pivotalURL                = "https://www.pivotaltracker.com"
	pivotalAPIURL             = pivotalURL + "/services/v5"
	pivotalStoryURL           = pivotalURL + "/story/show"
	pivotalIDRegexp           = `\[#(\d+)\]`
	githubAPIURL              = "https://api.github.com"
	githubAPIToken            = "GITHUB_API_TOKEN"
	githubCommitCompareAPIURL = githubAPIURL + "/repos/%s/%s/compare/%s...%s?access_token=%s" // owner, repo, hash1, hash2, api token
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

func appendUnique(lst []string, item string) []string {
	for _, elem := range lst {
		if elem == item {
			return lst
		}
	}
	return append(lst, item)
}

func getPivotalID(msg string) (string, error) {
	re := regexp.MustCompile(pivotalIDRegexp)
	strs := re.FindStringSubmatch(msg)
	if len(strs) <= 0 {
		return "", fmt.Errorf("Failed to locate pivotal id from message: %s", msg)
	}
	return strs[1], nil // first match
}

func (c StoryColumn) GetPivotalIDsFromGithubCommits() []string {
	// mocking list of commit messages for now. need to make calls to Github for actual commit messages
	var msgs = []string{"[#91474854] get ids", "[#93073292] skdajskfjsakjfks"}
	var ids []string
	for _, msg := range msgs {
		id, err := getPivotalID(msg)
		if err != nil {
			continue
		}
		ids = appendUnique(ids, id) // only get unique pivotal IDs
	}
	return ids
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
	var ptag = "<p>%s</p>"
	var content = ""
	var infoTmpl = "<a href=\"%s/%s\" target=\"_blank\">%s</a> %s<br/>"
	pivotalIDs := c.GetPivotalIDsFromGithubCommits()
	for _, ticket := range pivotalIDs {
		state := c.GetPivotalStoryStatus(ticket)
		status := fmt.Sprintf(bootstrapLabel["_base"], bootstrapLabel[state], state)
		info := fmt.Sprintf(infoTmpl, pivotalStoryURL, ticket, ticket, status)
		content += info
	}
	ptag = fmt.Sprintf(ptag, content)
	return template.HTML(fmt.Sprintf("<td>%s</td>", ptag)), nil
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
