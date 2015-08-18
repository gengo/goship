package acl

import (
	githublib "github.com/gengo/goship/lib/github"
	"github.com/golang/glog"
)

type githubAccessControl struct {
	gcl githublib.Client
}

// NewGithub returns an AccessControl which determines permissions in goship by permissions in github.
func NewGithub(gcl githublib.Client) AccessControl {
	return githubAccessControl{gcl: gcl}
}

// Readable determines if "user" has read permission on the repository "$owner/$repo".
func (ga githubAccessControl) Readable(owner, repo, user string) bool {
	m, _, err := ga.gcl.IsCollaborator(owner, repo, user)
	if err != nil {
		glog.Errorf("Failed to get Collaboration Status of User: %s %s %s err: %s", owner, user, repo, err)
		return false
	}
	return m
}

// Deployable determines if "user" is a member of a team which has write permission on the repository.
func (ga githubAccessControl) Deployable(owner, repo, user string) bool {
	// List the  all the teams for a repository.
	teams, _, err := ga.gcl.ListTeams(owner, repo, nil)
	if err != nil {
		glog.Errorf("Failed to get Organizations List: %s", err)
		return false
	}
	// Iterate through the teams for a repo, if a user is a member of a non read-only team exit with false.
	for _, team := range teams {
		o, _, err := ga.gcl.IsTeamMember(*team.ID, user)
		if err != nil {
			glog.Errorf("Failed to get Is Team Member from Org: %v", err)
			return false
		}
		// if user is a member of a non read only team return false
		if o == true && *team.Permission != "pull" {
			return true
		}

	}
	return false
}
