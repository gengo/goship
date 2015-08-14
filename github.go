package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	goship "github.com/gengo/goship/lib"
	"github.com/gengo/goship/lib/acl"
	"github.com/gengo/goship/lib/auth"
	githublib "github.com/gengo/goship/lib/github"
	"github.com/gengo/goship/lib/ssh"
	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"golang.org/x/net/context"
)

// latestDeployedCommit gets the latest commit hash on the host.
func latestDeployedCommit(username, hostname, repoPath string) ([]byte, error) {
	p, err := ioutil.ReadFile(*keyPath)
	if err != nil {
		return nil, errors.New("Failed to open private key file: " + err.Error())
	}
	return ssh.RemoteCmdOutput(context.TODO(), username, hostname, fmt.Sprintf("git --git-dir=%s rev-parse HEAD", repoPath), p)
}

// getCommit returns the latest commit deployed in the host.
func getCommit(host goship.Host, deployUser, repoPath string) (string, error) {
	lc, err := latestDeployedCommit(deployUser, host.URI+":"+sshPort, repoPath)
	if err != nil {
		glog.Errorf("Failed to get latest deployed commit: %s, %s. Error: %v", host.URI, deployUser, err)
		return "", err
	}
	return strings.TrimSpace(string(lc)), nil
}

// getLatestGitHubCommit returns the latest commit of the given branch.
func getLatestGitHubCommit(gcl githublib.Client, repoOwner, repoName, branch string) (string, error) {
	opts := &github.CommitsListOptions{SHA: branch}
	commits, _, err := gcl.ListCommits(repoOwner, repoName, opts)
	if err != nil {
		glog.Errorf("Failed to get commits from GitHub: %v", err)
		return "", err
	}
	return *commits[0].SHA, nil
}

// retrieveCommits fetches the latest deployed commits as well
// as the latest GitHub commits for a given Project.
// it will also check if the user has permission to pull.
func retrieveCommits(gcl githublib.Client, ac acl.AccessControl, r *http.Request, project goship.Project, deployUser string) (goship.Project, error) {
	// define a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	for i, environment := range project.Environments {
		for j, host := range environment.Hosts {
			// start a goroutine for SSHing on to the machine
			wg.Add(1)
			go func(i, j int, host goship.Host, repoPath string) {
				defer wg.Done()
				commit, err := getCommit(host, deployUser, repoPath)
				if err != nil {
					project.Environments[i].Hosts[j].LatestCommit = ""
					return
				}
				project.Environments[i].Hosts[j].LatestCommit = commit
			}(i, j, host, environment.RepoPath)
		}
		wg.Add(1)
		go func(i int, branch string) {
			defer wg.Done()
			commit, err := getLatestGitHubCommit(gcl, project.RepoOwner, project.RepoName, branch)
			if err != nil {
				project.Environments[i].LatestGitHubCommit = ""
				return
			}
			project.Environments[i].LatestGitHubCommit = commit
		}(i, environment.Branch)
	}
	// wait for goroutines to finish
	wg.Wait()
	for i, e := range project.Environments {
		project.Environments[i] = e
		for j, host := range e.Hosts {
			host.GitHubCommitURL = host.LatestGitHubCommitURL(project)
			host.GitHubDiffURL = host.LatestGitHubDiffURL(project, e)
			host.ShortCommitHash = host.LatestShortCommitHash()
			project.Environments[i].Hosts[j] = host
		}
	}
	u, err := auth.CurrentUser(r)
	if err != nil {
		glog.Errorf("Failed to get user %s", err)
	}
	return filterProject(ac, project, r, u), err
}

//  set projects to lock where user is only in a pull only repo and append a comment
func filterProject(ac acl.AccessControl, p goship.Project, r *http.Request, u auth.User) goship.Project {
	for i, e := range p.Environments {
		// If the repo isn't already locked.. lock it if the user doesnt have permission
		// and add to the comments
		if e.IsLocked {
			if p.Environments[i].Comment != "" {
				p.Environments[i].Comment = p.Environments[i].Comment + " | "
			}
			p.Environments[i].Comment = p.Environments[i].Comment + "repo is locked."
			continue
		}

		locked := !ac.Deployable(p.RepoOwner, p.RepoName, u.Name)
		p.Environments[i].IsLocked = locked
		// Add a line break if there is already a comment
		if p.Environments[i].Comment != "" {
			p.Environments[i].Comment = p.Environments[i].Comment + " | "
		}
		if locked {
			p.Environments[i].Comment = p.Environments[i].Comment + "you do not have permission to deploy "
		}
	}
	return p
}
