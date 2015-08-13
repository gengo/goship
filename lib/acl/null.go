package acl

type nullAccessControl struct{}

// Null is a null implementation of AccessControl.
var Null = AccessControl(nullAccessControl{})

// Readable always returns true
func (nullAccessControl) Readable(owner, repo, user string) bool {
	return true
}

// Deployable always returns true
func (nullAccessControl) Deployable(owner, repo, user string) bool {
	return true
}
