package gcr

import (
	"fmt"
)

// A Name is a name of a docker image.
type Name struct {
	// Registry is an optional registry part of the name.
	Registry string
	// NS is an optional namespace of the name.
	NS string
	// Name is the repository name
	Repo string
	// Tag is an optional tag of the name
	Tag string
}

// RepoWithNS returns a namespace-prefixed form of the name.
// It does not contain registry.
func (n Name) RepoWithNS() string {
	if n.NS == "" {
		return n.Repo
	}
	return fmt.Sprintf("%s/%s", n.NS, n.Repo)
}

func (n Name) RepoFullName() string {
	if n.Registry == "" {
		return n.RepoWithNS()
	}
	return fmt.Sprintf("%s/%s", n.Registry, n.RepoWithNS())
}

func (n Name) String() string {
	tag := n.Tag
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s:%s", n.RepoFullName(), tag)
}
