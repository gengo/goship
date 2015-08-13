package main

import (
	"log"

	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/auth"
	githublib "github.com/gengo/goship/lib/github"
)

type accessControl interface {
	readable(owner, repo, user string) bool
	deployable(owner, repo, user string) bool
}

type nullAccessControl struct{}

func (a nullAccessControl) readable(owner, repo, user string) bool {
	return true
}
func (a nullAccessControl) deployable(owner, repo, user string) bool {
	return true
}

type githubAccessControl struct {
	gcl githublib.Client
}

// readable determines if "user" has read permission on the repository "$owner/$repo".
// "owner" also can be an organization name but not only an user name.
func (ga githubAccessControl) readable(owner, repo, user string) bool {
	m, _, err := ga.gcl.IsCollaborator(owner, repo, user)
	if err != nil {
		log.Printf("Failure getting Collaboration Status of User: %s %s %s err: %s", owner, user, repo, err)
		return false
	}
	return m
}

func (ga githubAccessControl) deployable(owner, repo, user string) bool {
	// List the  all the teams for a repository.
	teams, _, err := ga.gcl.ListTeams(owner, repo, nil)
	if err != nil {
		log.Printf("Failure getting Organizations List %s", err)
		return false
	}
	// Iterate through the teams for a repo, if a user is a member of a non read-only team exit with false.
	for _, team := range teams {
		o, _, err := ga.gcl.IsTeamMember(*team.ID, user)
		if err != nil {
			log.Printf("\nFailure getting Is Team Member from Org \n [%s]", err)
			return false
		}
		// if user is a member of a non read only team return false
		if o == true && *team.Permission != "pull" {
			return true
		}

	}
	return false
}

func readableProjects(a accessControl, projects []goship.Project, u auth.User) []goship.Project {
	var readables []goship.Project
	for _, p := range projects {
		if a.readable(p.RepoOwner, p.RepoName, u.Name) {
			readables = append(readables, p)
		}
	}
	return readables
}
