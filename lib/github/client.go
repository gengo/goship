package github

import (
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

// Client is an interface for testability.
// It provides access to a subset of github APIs.
type Client interface {
	ListTeams(string, string, *github.ListOptions) ([]github.Team, *github.Response, error)
	ListCommits(owner, repo string, opts *github.CommitsListOptions) ([]github.RepositoryCommit, *github.Response, error)
	GetCommit(owner, repo, sha1 string) (*github.RepositoryCommit, *github.Response, error)
	IsTeamMember(int, string) (bool, *github.Response, error)
	IsCollaborator(string, string, string) (bool, *github.Response, error)
}

type prodClient struct {
	org  *github.OrganizationsService
	repo *github.RepositoriesService
}

// NewClient returns a new client of Github APIs.
// "token" must be a valid Github API access token with several scopes.
// TODO(yugui) Add a comprehensive list of the scopes.
func NewClient(token string) Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	c := github.NewClient(oauth2.NewClient(oauth2.NoContext, ts))
	return prodClient{
		org:  c.Organizations,
		repo: c.Repositories,
	}
}

// ListTeams exists in both organizations and repositories so we need to alias both functions
func (c prodClient) ListTeams(owner string, repo string, opt *github.ListOptions) ([]github.Team, *github.Response, error) {
	return c.repo.ListTeams(owner, repo, opt)
}

func (c prodClient) ListCommits(owner, repo string, opt *github.CommitsListOptions) ([]github.RepositoryCommit, *github.Response, error) {
	return c.repo.ListCommits(owner, repo, opt)
}

func (c prodClient) GetCommit(owner, repo, sha1 string) (*github.RepositoryCommit, *github.Response, error) {
	return c.repo.GetCommit(owner, repo, sha1)
}

func (c prodClient) IsTeamMember(team int, user string) (bool, *github.Response, error) {
	return c.org.IsTeamMember(team, user)
}

func (c prodClient) IsCollaborator(owner, repo, user string) (bool, *github.Response, error) {
	return c.repo.IsCollaborator(owner, repo, user)
}
