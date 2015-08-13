package acl

import (
	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/auth"
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
func ReadableProjects(a AccessControl, projects []goship.Project, u auth.User) []goship.Project {
	var readables []goship.Project
	for _, p := range projects {
		if a.Readable(p.RepoOwner, p.RepoName, u.Name) {
			readables = append(readables, p)
		}
	}
	return readables
}
