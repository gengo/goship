package githubtest

import (
	"fmt"

	githublib "github.com/gengo/goship/lib/github"
	"github.com/google/go-github/github"
)

// stub is a stub implementation of githublib.Client
// TODO(yugui) Accept stub values as struct fields but not hard-code them
type stub struct{}

func (s stub) ListTeams(owner string, repo string, opt *github.ListOptions) ([]github.Team, *github.Response, error) {
	a := github.Team{ID: github.Int(1), Name: github.String("team_1"), Permission: github.String("pull")}
	b := github.Team{ID: github.Int(2), Name: github.String("team_2"), Permission: github.String("push")}
	if repo == "repo_1" {
		return []github.Team{a}, nil, nil
	}
	if repo == "repo_2" {
		return []github.Team{b}, nil, nil
	}
	if repo == "repo_3" {
		return []github.Team{a, b}, nil, nil
	}
	return []github.Team{}, nil, nil
}

func (s stub) ListCommits(owner, repo string, opts *github.CommitsListOptions) ([]github.RepositoryCommit, *github.Response, error) {
	return nil, nil, fmt.Errorf("not implemented")
}

func (s stub) GetCommit(owner, repo, sha1 string) (*github.RepositoryCommit, *github.Response, error) {
	return nil, nil, fmt.Errorf("not implemented")
}

func (s stub) IsTeamMember(team int, user string) (bool, *github.Response, error) {
	if user == "read_only_user" && team == 1 {
		return true, nil, nil
	}
	if user == "push_user" && team == 2 {
		return true, nil, nil
	}
	if user == "push_and_pull_only_user" && (team == 1 || team == 2) {
		return true, nil, nil
	}
	return false, nil, nil
}

func (s stub) IsCollaborator(owner, repo, user string) (bool, *github.Response, error) {
	return true, nil, nil
}

func NewStub() githublib.Client {
	return stub{}
}
