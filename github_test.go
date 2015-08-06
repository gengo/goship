package main

import (
	"testing"

	"github.com/gengo/goship/lib/github/githubtest"
)

func TestUserOnNoTeam(t *testing.T) {
	g := githubtest.NewStub()
	authentication.authorization = true
	var want = false
	got, err := userHasDeployPermission(g, "owner_1", "repo_1", "read_only_user")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("User is Read Only = %v, want %v", got, want)
	}
}

func TestUserIsReadOnly(t *testing.T) {
	g := githubtest.NewStub()
	authentication.authorization = true
	var want = false
	got, err := userHasDeployPermission(g, "owner_1", "repo_1", "push_user")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("User is Read Only = %v, want %v", got, want)
	}
}

func TestUserHasPushPermission(t *testing.T) {
	g := githubtest.NewStub()
	var want = true
	got, err := userHasDeployPermission(g, "some_owner", "repo_2", "push_and_pull_only_user")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("User has Push Permission = %v, want %v", got, want)
	}
}

func TestPushPullUserHasPushPermission(t *testing.T) {
	g := githubtest.NewStub()
	var want = true
	got, err := userHasDeployPermission(g, "some_owner", "repo_3", "push_and_pull_only_user")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("User has Push Permission = %v, want %v", got, want)
	}
}
