package config

import (
	"fmt"
)

// SetComment will set the  comment field on an environment
func SetComment(client ETCDInterface, projectName, projectEnv, comment string) (err error) {
	projectString := fmt.Sprintf("/goship/projects/%s/environments/%s/comment", projectName, projectEnv)
	// guard against empty values ( simple validation)
	if projectName == "" || projectEnv == "" {
		return fmt.Errorf("Missing parameters")
	}
	_, err = client.Set(projectString, comment, 0)
	return err
}

// LockEnvironment Locks or unlock an environment for deploy
func LockEnvironment(client ETCDInterface, projectName, projectEnv, lock string) (err error) {
	projectString := fmt.Sprintf("/goship/projects/%s/environments/%s/locked", projectName, projectEnv)
	// guard against empty values ( simple validation)
	if projectName == "" || projectEnv == "" {
		return fmt.Errorf("Missing parameters")
	}
	_, err = client.Set(projectString, lock, 0)
	return err
}
