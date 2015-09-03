package config_test

import (
	"testing"

	"github.com/gengo/goship/lib/config"
)

func TestSetComment(t *testing.T) {
	const key = "/goship/projects/test_project/environments/test_environment/comment"
	ecl := mockEtcdClient{
		setExpectation: map[string]string{key: "A comment"},
	}
	err := config.SetComment(ecl, "test_project", "test_environment", "A comment")
	if err != nil {
		t.Fatalf("Can't set Comment %s", err)
	}
}

func TestLockingEnvironment(t *testing.T) {
	const key = "/goship/projects/test_project/environments/test_environment/locked"
	ecl := mockEtcdClient{
		setExpectation: map[string]string{key: "true"},
	}
	err := config.LockEnvironment(ecl, "test_project", "test_environment", "true")
	if err != nil {
		t.Fatalf("Can't lock %s", err)
	}
}

func TestUnlockingEnvironment(t *testing.T) {
	const key = "/goship/projects/test_project/environments/test_environment/locked"
	ecl := mockEtcdClient{
		setExpectation: map[string]string{key: "false"},
	}
	err := config.LockEnvironment(ecl, "test_project", "test_environment", "false")
	if err != nil {
		t.Fatalf("Can't unlock %s", err)
	}
}
