package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib"
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
	h    goship.Host
	p    goship.Project
	e    goship.Environment
	want string
}{
	{goship.Host{LatestCommit: "abc123"}, goship.Project{"test project", "https://github.com/test/foo", "foo", "test", []goship.Environment{}}, goship.Environment{LatestGitHubCommit: "abc123"}, ""},
	{goship.Host{LatestCommit: "abc123"}, goship.Project{"test project", "https://github.com/test/foo", "foo", "test", []goship.Environment{}}, goship.Environment{LatestGitHubCommit: "abc456"}, "https://github.com/test/foo/compare/abc123...abc456"},
}

func TestGitHubDiffURL(t *testing.T) {
	for _, tt := range githubDiffURLTests {
		if got := tt.h.LatestGitHubDiffURL(tt.p, tt.e); got != tt.want {
			t.Errorf("gitHubDiffURL = %s, want %s", got, tt.want)
		}
	}
}

var deployableTests = []struct {
	e    goship.Environment
	want bool
}{
	{goship.Environment{LatestGitHubCommit: "abc123", Hosts: []goship.Host{{LatestCommit: "abc456"}}}, true},
	{goship.Environment{LatestGitHubCommit: "abc456", Hosts: []goship.Host{{LatestCommit: "abc456"}}}, false},
}

func TestDeployable(t *testing.T) {
	for _, tt := range deployableTests {
		if got := tt.e.Deployable(); got != tt.want {
			t.Errorf("Deployable = %t, want %t", got, tt.want)
		}
	}
}

var wantConfig = goship.Config{
	Projects: []goship.Project{
		{Name: "Test Project One", GitHubURL: "https://github.com/test_owner/test_repo_name", RepoName: "test_repo_name", RepoOwner: "test_owner",
			Environments: []goship.Environment{{Name: "live", Deploy: "/deploy/test_project_one.sh", RepoPath: "/repos/test_repo_name/.git",
				Hosts: []goship.Host{{URI: "test-project-one.test.com"}}, Branch: "master", IsDeployable: false}}},
		{Name: "Test Project Two", GitHubURL: "https://github.com/test_owner/test_repo_name_two", RepoName: "test_repo_name_two", RepoOwner: "test_owner",
			Environments: []goship.Environment{{Name: "live", Deploy: "/deploy/test_project_two.sh", RepoPath: "/repos/test_repo_name_two/.git",
				Hosts: []goship.Host{{URI: "test-project-two.test.com"}}, Branch: "master", LatestGitHubCommit: "", IsDeployable: false}}}},
	DeployUser: "deploy_user",
	Notify:     "/notify/notify.sh",
	Pivotal:    &goship.PivotalConfiguration{Project: "111111", Token: "test"}}

func compareStrings(name, got, want string, t *testing.T) {
	if got != want {
		t.Errorf("got %s = %s; want %s", name, got, want)
	}
}

func TestCanParseETCD(t *testing.T) {

	got, err := goship.ParseETCD(&MockEtcdClient{})
	if err != nil {
		t.Fatalf("Can't parse %s %s", t, err)
	}
	compareStrings("deploy user", got.DeployUser, "test_user", t)
	compareStrings("token", got.Pivotal.Token, "XXXXXX", t)
	compareStrings("project", got.Pivotal.Project, "111111", t)
	compareStrings("project name", got.Projects[0].Name, "pivotal_project", t)
	compareStrings("repo path", got.Projects[0].Environments[0].RepoPath, "/repos/test_repo_name/.git", t)
	compareStrings("repo path", got.Projects[0].Environments[0].Branch, "master", t)
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
	var want = goship.Project{Name: "TestProject"}
	projects := []goship.Project{want}
	got, err := goship.ProjectFromName(projects, "TestProject")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Errorf("goship.GetProjectFromName = %v, want %v", got, want)
	}
	got, err = goship.ProjectFromName(projects, "BadProject")
	if err == nil {
		t.Errorf("goship.GetProjectFromName error case did not error", got, nil)
	}
}

func TestGetEnvironmentFromName(t *testing.T) {
	var (
		want = goship.Environment{Name: "TestEnvironment"}
		envs = []goship.Environment{want}
	)
	projects := []goship.Project{goship.Project{Name: "TestProject", Environments: envs}}
	got, err := goship.EnvironmentFromName(projects, "TestProject", "TestEnvironment")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, &want) {
		t.Errorf("goship.EnvironmentFromName = %v, want %v", got, want)
	}
	got, err = goship.EnvironmentFromName(projects, "BadProject", "BadEnvironment")
	if err == nil {
		t.Errorf("goship.EnvironmentFromName error case did not error")
	}
}

type MockEtcdClient struct{}

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

func TestReadEntries(t *testing.T) {
	var want = []DeployLogEntry{
		DeployLogEntry{DiffURL: "https://github.com/test/test/compare/firstsha...secondsha", ToRevisionMsg: "add test", User: "PlaceholderUser", Success: true, Time: time.Unix(1404362004, 0)},
		DeployLogEntry{DiffURL: "https://github.com/test/test/compare/thirdsha...fourthsha", ToRevisionMsg: "add another test", User: "PlaceholderUser", Success: false, Time: time.Unix(1404461492, 0)}}

	*dataPath = "testdata/deploylog"
	got, err := readEntries("Test-live")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readEntries = %v, want %v", got, want)
	}
}
