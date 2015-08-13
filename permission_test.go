package main

import (
	"testing"

	"github.com/gengo/goship/lib/github/githubtest"
)

func TestGithubAuthorizerDeployable(t *testing.T) {
	g := githubtest.NewStub()
	auth := githubAccessControl{gcl: g}
	for _, spec := range []struct {
		owner, repo, user string
		want              bool
	}{
		// See the implementation of githubtest.Stub for these stub values
		// TODO(yugui) Specify these values as parameters on instantiating "g".
		{
			owner: "owner_1", repo: "repo_1", user: "read_only_user",
			want: false,
		},
		{
			owner: "owner_1", repo: "repo_1", user: "push_user",
			want: false,
		},
		{
			owner: "some_owner", repo: "repo_2", user: "push_and_pull_only_user",
			want: true,
		},
		{
			owner: "some_owner", repo: "repo_3", user: "push_and_pull_only_user",
			want: true,
		},
	} {
		if got, want := auth.deployable(spec.owner, spec.repo, spec.user), spec.want; got != want {
			t.Errorf("auth.deployable(%q, %q, %q) = %v; want %v", spec.owner, spec.repo, spec.user, got, want)
		}
	}
}
