package main

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib"
	"github.com/google/go-github/github"
	"github.com/gorilla/sessions"
)

// create a github client interface so we can mock in tests
//type mockGithubClient interface {
//	ListTeams(string, string, *github.ListOptions) ([]github.Team, *github.Response, error)
//	IsTeamMember(int, string) (bool, *github.Response, error)
//}

type githubClientMock struct {
}

func (c githubClientMock) ListTeams(owner string, repo string, opt *github.ListOptions) ([]github.Team, *github.Response, error) {
	a := github.Team{ID: github.Int(1), Name: github.String("team_1"), Permission: github.String("pull")}
	b := github.Team{ID: github.Int(2), Name: github.String("team_2"), Permission: github.String("push")}
	if repo == "repo_1" {
		return []github.Team{a}, nil, nil
	} else if repo == "repo_2" {
		return []github.Team{b}, nil, nil
	} else if repo == "repo_3" {
		return []github.Team{a, b}, nil, nil
	}
	return []github.Team{}, nil, nil
}

func (c githubClientMock) IsTeamMember(team int, user string) (bool, *github.Response, error) {
	if user == "read_only_user" && team == 1 {
		return true, nil, nil
	} else if user == "push_user" && team == 2 {
		return true, nil, nil
	} else if user == "push_and_pull_only_user" && (team == 1 || team == 2) {
		return true, nil, nil
	}
	return false, nil, nil
}

func (c githubClientMock) IsCollaborator(owner, repo, user string) (bool, *github.Response, error) {
	return true, nil, nil
}

func newMockGithubClient() githubClientMock {
	return githubClientMock{}
}

func TestUserOnNoTeam(t *testing.T) {
	g := newMockGithubClient()
	authentication.authorization = true
	var want = false
	got, err := userHasDeployPermission(g, "owner_1", "repo_1", "read_only_user")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("User is Read Olny = %v, want %v", got, want)
	}
}

func TestUserIsReadOnly(t *testing.T) {
	g := newMockGithubClient()
	authentication.authorization = true
	var want = false
	got, err := userHasDeployPermission(g, "owner_1", "repo_1", "push_user")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("User is Read Olny = %v, want %v", got, want)
	}
}

func TestUserHasPushPermission(t *testing.T) {
	g := newMockGithubClient()
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
	g := newMockGithubClient()
	var want = true
	got, err := userHasDeployPermission(g, "some_owner", "repo_3", "push_and_pull_only_user")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("User has Push Permission = %v, want %v", got, want)
	}
}

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

func TestSetComment(t *testing.T) {
	err := goship.SetComment(&MockEtcdClient{}, "test_project", "test_environment", "A comment")
	if err != nil {
		t.Fatalf("Can't set Comment %s", err)
	}
}

func TestLockingEnvironment(t *testing.T) {
	err := goship.LockEnvironment(&MockEtcdClient{}, "test_project", "test_environment", "true")
	if err != nil {
		t.Fatalf("Can't lock %s", err)
	}
}

func TestUnlockingEnvironment(t *testing.T) {
	err := goship.LockEnvironment(&MockEtcdClient{}, "test_project", "test_environment", "false")
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

func TestCleanProjects(t *testing.T) {
	authentication.authorization = true
	req, _ := http.NewRequest("GET", "", nil)
	w := httptest.NewRecorder()
	HomeHandler(w, req)

	p, err := goship.ParseETCD(&MockEtcdClient{})
	if err != nil {
		t.Fatalf("Can't parse %s %s", t, err)
	}
	u := User{}
	u.UserName = "bob"

	got := len(p.Projects)
	if got < 1 {
		t.Errorf("clean projects test expects projects to have at least one project [%d]", got)
	}
	got = len(removeUnauthorizedProjects(p.Projects, req, u))
	if got != 0 {
		t.Errorf("clean projects failed to clean project for unauth user.. [%d]", got)
	}
}

func TestGetUser(t *testing.T) {
	authentication.authorization = true
	req, _ := http.NewRequest("GET", "", nil)
	w := httptest.NewRecorder()
	HomeHandler(w, req)
	session, err := store.Get(req, sessionName)
	if err != nil {
		t.Errorf("Can't get a session store")
	}
	session.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
	}

	session.Values["userName"] = "T-800"
	session.Values["avatarURL"] = "http://fake.com/1234"
	session.Save(req, w)
	user, err := getUser(req)
	if err != nil {
		t.Errorf("Failed to get User from GetUser [%s]", err)
	}
	if user.UserName != "T-800" {
		t.Errorf("Failed to get User Name, expected T-800 got [%s]", user.UserName)
	}
	if user.UserAvatar != "http://fake.com/1234" {
		t.Errorf("Failed to get User Avatar, expected http://fake.com/1234 got [%s]", user.UserAvatar)

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

func (*MockEtcdClient) Set(s, c string, x uint64) (*etcd.Response, error) {
	m := make(map[string]*etcd.Response)
	// {"action":"set",
	//           "node":
	//                  {"key":"/projects/admin/environments/staging/comment"
	//                  ,"value":"XXXXX","modifiedIndex":209,"createdIndex":209},
	//            "prevNode":
	//                   {"key":"/projects/admin/environments/staging/comment",
	//                    "value":"some comment","modifiedIndex":208,"createdIndex":208}}
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
