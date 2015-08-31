// Travis adds Travis build banners to Goship.
// For public repos, it should be automatic.
// For private repos, add your travis token to the project in ETCD
// etcdctl set /projects/{project_name}/travis_token {travis_token}
package travis

import (
	"fmt"
	"html/template"

	"github.com/gengo/goship/lib/config"
	"github.com/gengo/goship/plugins/plugin"
)

type TravisPlugin struct{}

func init() {
	var p TravisPlugin
	plugin.RegisterPlugin(p)
}

var rootUrls = []string{"https://travis-ci.org", "https://magnum.travis-ci.com"}

type TravisColumn struct {
	Project      string
	Token        string
	Organization string
}

func (c TravisColumn) RenderHeader() (template.HTML, error) {
	return template.HTML(`<th style="min-width: 100px">Build Status</th>`), nil
}

func (c TravisColumn) RenderDetail() (template.HTML, error) {
	var url, svg string
	if c.Token == "" {
		url = fmt.Sprintf("%s/%s/%s", rootUrls[0], c.Organization, c.Project)
		svg = fmt.Sprintf("%s/%s/%s.svg?branch=master", rootUrls[0], c.Organization, c.Project)
	} else {
		url = fmt.Sprintf("%s/%s/%s", rootUrls[1], c.Organization, c.Project)
		svg = fmt.Sprintf("%s/%s/%s.svg?token=%s&branch=master", rootUrls[1], c.Organization, c.Project, c.Token)
	}
	return template.HTML(fmt.Sprintf(`<td><a target=_blank href=%s><img src=%s onerror='this.style.display = "none"'></img></a></td>`, url, svg)), nil
}

func (p TravisPlugin) Apply(proj config.Project) ([]plugin.Column, error) {
	c := TravisColumn{
		Project:      proj.RepoName,
		Token:        proj.TravisToken,
		Organization: proj.RepoOwner,
	}
	return []plugin.Column{c}, nil
}
