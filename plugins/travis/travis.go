// Travis adds Travis build banners to Goship.
// For public repos, it should be automatic.
// For private repos, add your travis token to the project in ETCD
// etcdctl set /projects/{project_name}/travis_token {travis_token}
package travis

import (
	"fmt"
	"html/template"
	"log"

	goship "github.com/gengo/goship/lib"

	"github.com/gengo/goship/plugins/plugin"
)

type TravisPlugin struct{}

func init() {
	p := &TravisPlugin{}
	plugin.RegisterPlugin(p)
}

var rootUrls = []string{"https://travis-ci.org", "https://magnum.travis-ci.com"}

type TravisColumn struct {
	Project      string
	Token        string
	Organization string
}

func (c TravisColumn) RenderHeader() (template.HTML, error) {
	return template.HTML("<th>Build Status</th>"), nil
}

func (c TravisColumn) RenderDetail(e goship.Environment) (template.HTML, error) {
	var url, svg string
	if c.Token == "" {
		url = fmt.Sprintf("%s/%s/%s", rootUrls[0], c.Organization, c.Project)
		svg = fmt.Sprintf("%s/%s/%s.svg?branch=master", rootUrls[0], c.Organization, c.Project)
	} else {
		url = fmt.Sprintf("%s/%s/%s", rootUrls[1], c.Organization, c.Project)
		svg = fmt.Sprintf("%s/%s/%s.svg?token=%s&branch=master", rootUrls[1], c.Organization, c.Project, c.Token)
	}
	return template.HTML(fmt.Sprintf("<td><a target=_blank href=%s><img src=%s></img></a></td>", url, svg)), nil
}

func getToken(c goship.ETCDInterface, p goship.Project) string {
	r, err := c.Get(fmt.Sprintf("/projects/%s/travis_token", p.Name), false, false)
	if err != nil {
		log.Print(err)
		return ""
	}
	return r.Node.Value
}

func (p *TravisPlugin) Apply(c goship.Config) error {
	for i := range c.Projects {
		c.Projects[i].AddPluginColumn(TravisColumn{
			Project:      c.Projects[i].RepoName,
			Token:        getToken(c.ETCDClient, c.Projects[i]),
			Organization: c.Projects[i].RepoOwner,
		})
	}
	return nil
}
