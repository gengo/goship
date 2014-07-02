package main

import "testing"

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
			t.Errorf("Deployable = %s, want %s", got, tt.want)
		}
	}
}
