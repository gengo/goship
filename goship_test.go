package main

import (
	"reflect"
	"testing"
)

func TestStripANSICodes(t *testing.T) {
	tests := []struct {
		give string
		want string
	}{
		{"\x1B[0m", ""},
		{"some text \x1B[23m\x1B[2;13m", "some text "},
		{"no code", "no code"},
		{"\x1B[13m\x1B[23m\x1B[3m", ""},
	}

	for _, tt := range tests {
		got := stripANSICodes(tt.give)
		if got != tt.want {
			t.Errorf("stripANSICodes(%q) got %q, want %q", tt.give, got, tt.want)
		}
	}
}

var githubDiffURLTests = []struct {
	h    Host
	p    Project
	e    Environment
	want string
}{
	{Host{LatestCommit: "abc123"}, Project{"test project", "https://github.com/test/foo", "foo", "test", []Environment{}}, Environment{LatestGitHubCommit: "abc123"}, ""},
	{Host{LatestCommit: "abc123"}, Project{"test project", "https://github.com/test/foo", "foo", "test", []Environment{}}, Environment{LatestGitHubCommit: "abc456"}, "https://github.com/test/foo/compare/abc123...abc456"},
}

func TestGitHubDiffURL(t *testing.T) {
	for _, tt := range githubDiffURLTests {
		if got := tt.h.gitHubDiffURL(tt.p, tt.e); got != tt.want {
			t.Errorf("gitHubDiffURL = %s, want %s", got, tt.want)
		}
	}
}

var deployableTests = []struct {
	e    Environment
	want bool
}{
	{Environment{LatestGitHubCommit: "abc123", Hosts: []Host{{LatestCommit: "abc456"}}}, true},
	{Environment{LatestGitHubCommit: "abc456", Hosts: []Host{{LatestCommit: "abc456"}}}, false},
}

func TestDeployable(t *testing.T) {
	for _, tt := range deployableTests {
		if got := tt.e.Deployable(); got != tt.want {
			t.Errorf("Deployable = %t, want %t", got, tt.want)
		}
	}
}

var wantConfig = config{
	Projects: []Project{
		{Name: "Test Project One", GitHubURL: "https://github.com/test_owner/test_repo_name", RepoName: "test_repo_name", RepoOwner: "test_owner",
			Environments: []Environment{{Name: "live", Deploy: "/deploy/test_project_one.sh", RepoPath: "/repos/test_repo_name/.git",
				Hosts: []Host{{URI: "test-project-one.test.com"}}, Branch: "master", IsDeployable: false}}},
		{Name: "Test Project Two", GitHubURL: "https://github.com/test_owner/test_repo_name_two", RepoName: "test_repo_name_two", RepoOwner: "test_owner",
			Environments: []Environment{{Name: "live", Deploy: "/deploy/test_project_two.sh", RepoPath: "/repos/test_repo_name_two/.git",
				Hosts: []Host{{URI: "test-project-two.test.com"}}, Branch: "master", LatestGitHubCommit: "", IsDeployable: false}}}},
	DeployUser: "deploy_user",
	Notify:     "/notify/notify.sh",
	Pivotal:    &PivotalConfiguration{project: "111111", token: "test"}}

func TestParseYAML(t *testing.T) {
	// set the configFile flag for test
	*configFile = "testdata/config.yml"
	got, err := parseYAML()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, wantConfig) {
		t.Errorf("parseYAML = %v\n, want %v", got, wantConfig)
	}
}
