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
)

// latestDeployedCommit gets the latest commit hash on the host.
func latestDeployedCommit(username, hostname string, e goship.Environment) (b []byte, err error) {
	p, err := ioutil.ReadFile(*keyPath)
	if err != nil {
		return b, errors.New("Failed to open private key file: " + err.Error())
	}
	o, err := ssh.RemoteCmdOutput(username, hostname, fmt.Sprintf("git --git-dir=%s rev-parse HEAD", e.RepoPath), p)
	if err != nil {
		return b, err
	}
	return o, nil
}

// getCommit is called in a goroutine and gets the latest deployed commit on a host.
// It updates the Environment in-place.
func getCommit(wg *sync.WaitGroup, project goship.Project, env goship.Environment, host goship.Host, deployUser string, i, j int) {
	defer wg.Done()
	lc, err := latestDeployedCommit(deployUser, host.URI+":"+sshPort, env)
	if err != nil {
		glog.Errorf("Failed to get latest deployed commit: %s, %s. Error: %v", host.URI, deployUser, err)
		host.LatestCommit = string(lc)
		project.Environments[i].Hosts[j] = host
	}
	host.LatestCommit = strings.TrimSpace(string(lc))
	project.Environments[i].Hosts[j] = host
}

// getLatestGitHubCommit is called in a goroutine and retrieves the latest commit
// from GitHub for a given branch of a project. It updates the Environment in-place.
func getLatestGitHubCommit(wg *sync.WaitGroup, project goship.Project, environment goship.Environment, gcl githublib.Client, repoOwner, repoName string, i int) {
	defer wg.Done()
	opts := &github.CommitsListOptions{SHA: environment.Branch}
	commits, _, err := gcl.ListCommits(repoOwner, repoName, opts)
	if err != nil {
		glog.Errorf("Failed to get commits from GitHub: %v", err)
		environment.LatestGitHubCommit = ""
	} else {
		environment.LatestGitHubCommit = *commits[0].SHA
	}
	project.Environments[i] = environment
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
			go getCommit(&wg, project, environment, host, deployUser, i, j)
		}
		wg.Add(1)
		go getLatestGitHubCommit(&wg, project, environment, gcl, project.RepoOwner, project.RepoName, i)
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
