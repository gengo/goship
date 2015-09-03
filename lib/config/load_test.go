package config_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib/config"
)

func TestLoad(t *testing.T) {
	ecl := mockEtcdClient{
		getExpectation: map[string]*etcd.Node{
			"/goship/config": &etcd.Node{
				Key: "/goship/config",
				Value: `
					{
						"deploy_user": "test_user",
						"notify": "notify-command",
						"pivotal": {
							"project": "11111",
							"token": "pivotal token"
						}
					}
				`,
			},
			"/goship/projects": &etcd.Node{
				Key: "/goship/projects",
				Dir: true,
				Nodes: etcd.Nodes{
					{
						Key: "/goship/projects/example-project",
						Dir: true,
						Nodes: etcd.Nodes{
							{
								Key: "/goship/projects/example-project/config",
								Value: `
									{
										"repo_name": "example",
										"repo_owner": "gengo",
										"travis_token": "example_token"
									}
								`,
							},
							{
								Key: "/goship/projects/example-project/environments",
								Dir: true,
								Nodes: etcd.Nodes{
									{
										Key: "/goship/projects/example-project/environments/example-environment",
										Value: `
											{
												"deploy": "deploy-command",
												"repo_path": "/path/to/prod",
												"hosts": [ "host1", "host2", "host3" ]
											}
										`,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	got, err := config.Load(ecl)
	if err != nil {
		t.Errorf("config.Load(%v) failed with %v; want success", ecl, err)
		return
	}
	want := config.Config{
		DeployUser: "test_user",
		Notify:     "notify-command",
		Pivotal: &config.PivotalConfiguration{
			Token: "pivotal token",
		},
		Projects: []config.Project{
			{
				Name:      "example-project",
				RepoName:  "example",
				RepoOwner: "gengo",
				Environments: []config.Environment{
					{
						Name:     "example-environment",
						Deploy:   "deploy-command",
						RepoPath: "/path/to/prod",
						Branch:   "master",
						Hosts:    []string{"host1", "host2", "host3"},
					},
				},
				TravisToken: "example_token",
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("config.Load(ecl) = %#v; want %#v", got, want)
	}
}

type mockEtcdClient struct {
	setExpectation map[string]string
	getExpectation map[string]*etcd.Node
}

func (cl mockEtcdClient) Set(key, value string, ttl uint64) (*etcd.Response, error) {
	v, ok := cl.setExpectation[key]
	if !ok {
		return nil, fmt.Errorf("unexpected key %q", key)
	}
	if got, want := v, value; got != want {
		return nil, fmt.Errorf("value=%q; want %q", got, want)
	}
	return &etcd.Response{
		Action:   "Set",
		Node:     &etcd.Node{Key: key, Value: value},
		PrevNode: &etcd.Node{Key: key, Value: "something else"},
	}, nil
}

func (cl mockEtcdClient) Get(key string, sort bool, recursive bool) (*etcd.Response, error) {
	node, ok := cl.getExpectation[key]
	if !ok {
		return nil, fmt.Errorf("unexpected key %q", key)
	}
	return &etcd.Response{
		Action:    "Get",
		Node:      node,
		EtcdIndex: 1,
		RaftIndex: 1,
		RaftTerm:  1,
	}, nil
}
