package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/config"
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
	h    config.Host
	p    config.Project
	e    config.Environment
	want string
}{
	{config.Host{LatestCommit: "abc123"}, config.Project{nil, "test project", "https://github.com/test/foo", "foo", "test", []config.Environment{}}, config.Environment{LatestGitHubCommit: "abc123"}, ""},
	{config.Host{LatestCommit: "abc123"}, config.Project{nil, "test project", "https://github.com/test/foo", "foo", "test", []config.Environment{}}, config.Environment{LatestGitHubCommit: "abc456"}, "https://github.com/test/foo/compare/abc123...abc456"},
}

func TestGitHubDiffURL(t *testing.T) {
	for _, tt := range githubDiffURLTests {
		if got := tt.h.LatestGitHubDiffURL(tt.p, tt.e); got != tt.want {
			t.Errorf("gitHubDiffURL = %s, want %s", got, tt.want)
		}
	}
}

var wantConfig = config.Config{
	Projects: []config.Project{
		{Name: "Test Project One", GitHubURL: "https://github.com/test_owner/test_repo_name", RepoName: "test_repo_name", RepoOwner: "test_owner",
			Environments: []config.Environment{{Name: "live", Deploy: "/deploy/test_project_one.sh", RepoPath: "/repos/test_repo_name/.git",
				Hosts: []config.Host{{URI: "test-project-one.test.com"}}, Branch: "master"}}},
		{Name: "Test Project Two", GitHubURL: "https://github.com/test_owner/test_repo_name_two", RepoName: "test_repo_name_two", RepoOwner: "test_owner",
			Environments: []config.Environment{{Name: "live", Deploy: "/deploy/test_project_two.sh", RepoPath: "/repos/test_repo_name_two/.git",
				Hosts: []config.Host{{URI: "test-project-two.test.com"}}, Branch: "master", LatestGitHubCommit: ""}}}},
	DeployUser: "deploy_user",
	Notify:     "/notify/notify.sh",
	Pivotal:    &config.PivotalConfiguration{Project: "111111", Token: "test"}}

func TestSetComment(t *testing.T) {
	err := config.SetComment(&MockEtcdClient{}, "test_project", "test_environment", "A comment")
	if err != nil {
		t.Fatalf("Can't set Comment %s", err)
	}
}

func TestLockingEnvironment(t *testing.T) {
	err := config.LockEnvironment(&MockEtcdClient{}, "test_project", "test_environment", "true")
	if err != nil {
		t.Fatalf("Can't lock %s", err)
	}
}

func TestUnlockingEnvironment(t *testing.T) {
	err := config.LockEnvironment(&MockEtcdClient{}, "test_project", "test_environment", "false")
	if err != nil {
		t.Fatalf("Can't unlock %s", err)
	}
}

func compareStrings(name, got, want string, t *testing.T) {
	if got != want {
		t.Errorf("got %s = %s; want %s", name, got, want)
	}
}

func TestCanParseETCD(t *testing.T) {

	got, err := config.ParseETCD(&MockEtcdClient{})
	if err != nil {
		t.Fatalf("Can't parse %s %s", t, err)
	}
	compareStrings("deploy user", got.DeployUser, "test_user", t)
	compareStrings("token", got.Pivotal.Token, "XXXXXX", t)
	compareStrings("project", got.Pivotal.Project, "111111", t)
	compareStrings("project name", got.Projects[0].Name, "pivotal_project", t)
	compareStrings("repo path", got.Projects[0].Environments[0].RepoPath, "/repos/test_repo_name/.git", t)
	compareStrings("repo branch", got.Projects[0].Environments[0].Branch, "master", t)
	compareStrings("host name", got.Projects[0].Environments[0].Hosts[0].URI, "test-qa-01.somewhere.com", t)
}

var now = time.Now()

var formatTimeTests = []struct {
	t    time.Time
	want string
}{
	{time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC), "Nov 10, 2009 at 11:00pm (UTC)"},
	{time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second()-1, 0, now.Location()), "1 second ago"},
	{time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second()-30, 0, now.Location()), "30 seconds ago"},
	{time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-1, now.Second(), 0, now.Location()), "1 minute ago"},
	{time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute()-30, now.Second(), 0, now.Location()), "30 minutes ago"},
	{time.Date(now.Year(), now.Month(), now.Day(), now.Hour()-1, now.Minute(), now.Second(), 0, now.Location()), "1 hour ago"},
	{time.Date(now.Year(), now.Month(), now.Day(), now.Hour()-3, now.Minute(), now.Second(), 0, now.Location()), "3 hours ago"},
}

func TestFormatTime(t *testing.T) {
	for _, tt := range formatTimeTests {
		if got := formatTime(tt.t); got != tt.want {
			t.Errorf("formatTime(%s) = %s, want %s", tt.t, got, tt.want)
		}
	}
}

func TestProjectFromName(t *testing.T) {
	var want = config.Project{Name: "TestProject"}
	projects := []config.Project{want}
	got, err := config.ProjectFromName(projects, "TestProject")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Errorf("config.GetProjectFromName = %v, want %v", got, want)
	}
	got, err = config.ProjectFromName(projects, "BadProject")
	if err == nil {
		t.Errorf("config.GetProjectFromName error case did not error")
	}
}

func TestCleanProjects(t *testing.T) {
	// Tenatively disabled because there's no way to safely test the target function
	// TODO(yugui) recover this test once we make the github client mockable.
	return
	/*
		req, _ := http.NewRequest("GET", "", nil)

		p, err := config.ParseETCD(&MockEtcdClient{})
		if err != nil {
			t.Fatalf("Can't parse %s %s", t, err)
		}
		u := auth.User{
			Name: "bob",
		}

		got := len(p.Projects)
		if got < 1 {
			t.Errorf("clean projects test expects projects to have at least one project [%d]", got)
		}
		got = len(removeUnauthorizedProjects(p.Projects, req, u))
		if got != 0 {
			t.Errorf("clean projects failed to clean project for unauth user.. [%d]", got)
		}
	*/
}

func TestGetEnvironmentFromName(t *testing.T) {
	var (
		want = config.Environment{Name: "TestEnvironment"}
		envs = []config.Environment{want}
	)
	projects := []config.Project{config.Project{Name: "TestProject", Environments: envs}}
	got, err := config.EnvironmentFromName(projects, "TestProject", "TestEnvironment")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Errorf("config.EnvironmentFromName = %v, want %v", got, want)
	}
	got, err = config.EnvironmentFromName(projects, "BadProject", "BadEnvironment")
	if err == nil {
		t.Errorf("config.EnvironmentFromName error case did not error")
	}
}

type MockEtcdClient struct{}

func (*MockEtcdClient) Set(s, c string, x uint64) (*etcd.Response, error) {
	m := make(map[string]*etcd.Response)
	m["/projects/test_project/environments/test_environment/comment"] = &etcd.Response{
		Action: "Set",
		Node: &etcd.Node{
			Key: "/projects/test_project/environments/test_environment/", Value: "XXXX",
		},
		PrevNode: &etcd.Node{
			Key: "/projects/test_project/environments/test_environment/", Value: "YYYY",
		},
	}
	mockResponse := m[s]
	return mockResponse, nil
}

//Mock calls to ETCD here. Each etcd Response should return the structs you need.
func (*MockEtcdClient) Get(s string, t bool, x bool) (*etcd.Response, error) {
	m := make(map[string]*etcd.Response)
	m["/"] = &etcd.Response{Action: "Get", Node: &etcd.Node{
		Key: "projects", Value: "",
		Nodes: etcd.Nodes{
			{Key: "deploy_user", Value: "test_user", Dir: false},
			{Key: "pivotal_token", Value: "XXXXXX", Dir: false},
			{Key: "pivotal_project", Value: "111111", Dir: false},
		}, Dir: true,
	}, EtcdIndex: 1, RaftIndex: 1, RaftTerm: 1,
	}
	m["/projects"] = &etcd.Response{
		Action: "Get",
		Node: &etcd.Node{
			Key: "projects", Value: "",
			Nodes: etcd.Nodes{
				{Key: "/projects/pivotal_project", Dir: true},
			}, Dir: true,
		},
		EtcdIndex: 1, RaftIndex: 1, RaftTerm: 1,
	}
	m["/projects/pivotal_project"] = &etcd.Response{
		Action: "Get",
		Node: &etcd.Node{
			Key: "/projects/pivotal_project", Value: "",
			Nodes: etcd.Nodes{
				{
					Key: "project_name", Value: "/projects/pivotal_project/project_name/TC", Dir: true,
				},
			},
			Dir: true,
		},
		EtcdIndex: 1, RaftIndex: 1, RaftTerm: 1,
	}
	m["/projects/pivotal_project/environments"] = &etcd.Response{
		Action: "Get",
		Node: &etcd.Node{
			Key:   "/projects/pivotal_project/environments",
			Value: "",
			Nodes: etcd.Nodes{
				{Key: "qa", Dir: true},
			},
			Dir: true,
		},
		EtcdIndex: 1, RaftIndex: 1, RaftTerm: 1,
	}
	m["/projects/pivotal_project/environments/qa"] = &etcd.Response{
		Action: "Get",
		Node: &etcd.Node{
			Key:   "qa",
			Value: "",
			Nodes: etcd.Nodes{
				{Key: "repo_path", Value: "/repos/test_repo_name/.git", Dir: false},
				{Key: "branch", Value: "master", Dir: false},
			}, Dir: true,
		}, EtcdIndex: 1, RaftIndex: 1, RaftTerm: 1,
	}
	m["/projects/pivotal_project/environments/qa/hosts"] = &etcd.Response{
		Action: "Get",
		Node: &etcd.Node{
			Key: "/projects/pivotal_project/environments/qa/hosts", Value: "",
			Nodes: etcd.Nodes{
				{Key: "test-qa-01.somewhere.com", Dir: true},
			},
			Dir: true,
		},
		EtcdIndex: 1, RaftIndex: 1, RaftTerm: 1,
	}
	mockResponse := m[s]
	return mockResponse, nil
}
