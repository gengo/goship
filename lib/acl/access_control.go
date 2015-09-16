package acl

import (
	"github.com/gengo/goship/lib/auth"
	"github.com/gengo/goship/lib/config"
	"github.com/golang/glog"
)

// AccessControl provides permission check of end-users on resources/operations in goship
type AccessControl interface {
	// Readable determines if "user" is allowed to read the repository
	// "owner" also can be an organization name but not only an user name.
	Readable(owner, repo, user string) bool
	// Deployable determines if "user" is allowed to deploy from the repository
	Deployable(owner, repo, user string) bool
}

// ReadableProjects filters a list of projects.
// It returns a new list of projects whose items are in "projects" and readable by "u".
func ReadableProjects(a AccessControl, projects []config.Project, u auth.User) []config.Project {
	var readables []config.Project
	for _, p := range projects {
		repo := p.SourceRepo()
		if a.Readable(repo.RepoOwner, repo.RepoName, u.Name) {
			glog.V(2).Infof("%s/%s is readable for %s", p.RepoOwner, p.RepoName, u.Name)
			readables = append(readables, p)
		} else {
			glog.V(1).Infof("%s/%s is not readable for %s", p.RepoOwner, p.RepoName, u.Name)
		}
	}
	return readables
}
