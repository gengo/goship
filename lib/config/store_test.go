package config_test

import (
	"encoding/json"
	"testing"

	"github.com/gengo/goship/lib/config"
)

func TestStore(t *testing.T) {
	cfg := config.Config{
		DeployUser: "test_user",
		Notify:     "notify-command",
		Pivotal: &config.PivotalConfiguration{
			Token: "pivotal token",
		},
		Projects: []config.Project{
			{
				Name: "example-project",
				Repo: config.Repo{
					RepoName:  "example",
					RepoOwner: "gengo",
				},
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
	marshal := func(obj interface{}) string {
		buf, err := json.Marshal(obj)
		if err != nil {
			t.Fatalf("json.Marshal(%#v) failed with %v; want success", obj, err)
		}
		return string(buf)
	}

	ecl := mockEtcdClient{
		setExpectation: map[string]string{
			"/goship/config":                                                    marshal(cfg),
			"/goship/projects/example-project/config":                           marshal(cfg.Projects[0]),
			"/goship/projects/example-project/environments/example-environment": marshal(cfg.Projects[0].Environments[0]),
		},
	}

	if err := config.Store(ecl, cfg); err != nil {
		t.Errorf("config.Store(ecl, %#v) failed with %v; want success", cfg, err)
	}
}
