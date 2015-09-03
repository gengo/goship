package config_test

import (
	"reflect"
	"testing"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/config"
)

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

func TestLoad(t *testing.T) {
	got, err := config.Load(&MockEtcdClient{})
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

func TestProjectFromName(t *testing.T) {
	var want = config.Project{Name: "TestProject"}
	projects := []config.Project{want}
	got, err := config.ProjectFromName(projects, "TestProject")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
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
		Key: "/", Value: "",
		Nodes: etcd.Nodes{
			{Key: "/deploy_user", Value: "test_user", Dir: false},
			{Key: "/pivotal_token", Value: "XXXXXX", Dir: false},
			{Key: "/pivotal_project", Value: "111111", Dir: false},
		}, Dir: true,
	}, EtcdIndex: 1, RaftIndex: 1, RaftTerm: 1,
	}
	m["/projects"] = &etcd.Response{
		Action: "Get",
		Node: &etcd.Node{
			Key: "/projects", Value: "",
			Nodes: etcd.Nodes{
				{
					Key: "/projects/pivotal_project",
					Dir: true,
					Nodes: etcd.Nodes{
						{
							Key:   "/projects/pivotal_project/project_name",
							Value: "/projects/pivotal_project/project_name/TC",
							Dir:   true,
						},
						{
							Key: "/projects/pivotal_project/environments",
							Dir: true,
							Nodes: etcd.Nodes{
								{
									Key: "/projects/pivotal_project/environments/qa",
									Dir: true,
									Nodes: etcd.Nodes{
										{Key: "/projects/pivotal_project/environments/qa/repo_path", Value: "/repos/test_repo_name/.git"},
										{Key: "/projects/pivotal_project/environments/qa/branch", Value: "master"},
										{
											Key: "/projects/pivotal_project/environments/qa/hosts",
											Dir: true,
											Nodes: etcd.Nodes{
												{Key: "test-qa-01.somewhere.com", Dir: true},
											},
										},
									},
								},
							},
						},
					},
				},
			}, Dir: true,
		},
		EtcdIndex: 1, RaftIndex: 1, RaftTerm: 1,
	}
	mockResponse := m[s]
	return mockResponse, nil
}
