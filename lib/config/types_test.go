package config_test

import (
	"reflect"
	"testing"

	"github.com/gengo/goship/lib/config"
)

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
