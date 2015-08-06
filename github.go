package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	goship "github.com/gengo/goship/lib"
	githublib "github.com/gengo/goship/lib/github"
	"github.com/google/go-github/github"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

const (
	gitHubAPITokenEnvVar = "GITHUB_API_TOKEN"
)

func newGithubClient() githublib.Client {
	gt := os.Getenv(gitHubAPITokenEnvVar)
	return githublib.NewClient(gt)
}

// remoteCmdOutput runs the given command on a remote server at the given hostname as the given user.
func remoteCmdOutput(username, hostname, cmd string, privateKey []byte) (b []byte, err error) {
	p, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return b, err
	}
	s, err := ssh.NewSignerFromKey(p)
	if err != nil {
		return b, err
	}
	pub := ssh.PublicKeys(s)
	clientConfig := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{pub},
	}
	client, err := ssh.Dial("tcp", hostname, clientConfig)
	if err != nil {
		return b, errors.New("ERROR: Failed to dial: " + err.Error())
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return b, errors.New("ERROR: Failed to create session: " + err.Error())
	}
	defer session.Close()
	b, err = session.Output(cmd)
	if err != nil {
		return b, fmt.Errorf("ERROR: Failed to run cmd on host %s: %s", hostname, err.Error())
	}
	return b, nil
}

// latestDeployedCommit gets the latest commit hash on the host.
func latestDeployedCommit(username, hostname string, e goship.Environment) (b []byte, err error) {
	p, err := ioutil.ReadFile(*keyPath)
	if err != nil {
		return b, errors.New("Failed to open private key file: " + err.Error())
	}
	o, err := remoteCmdOutput(username, hostname, fmt.Sprintf("git --git-dir=%s rev-parse HEAD", e.RepoPath), p)
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
		log.Printf("ERROR: failed to get latest deployed commit: %s, %s. Error: %v", host.URI, deployUser, err)
		host.LatestCommit = string(lc)
		project.Environments[i].Hosts[j] = host
	}
	host.LatestCommit = strings.TrimSpace(string(lc))
	project.Environments[i].Hosts[j] = host
}

// getLatestGitHubCommit is called in a goroutine and retrieves the latest commit
// from GitHub for a given branch of a project. It updates the Environment in-place.
func getLatestGitHubCommit(wg *sync.WaitGroup, project goship.Project, environment goship.Environment, c *github.Client, repoOwner, repoName string, i int) {
	defer wg.Done()
	opts := &github.CommitsListOptions{SHA: environment.Branch}
	commits, _, err := c.Repositories.ListCommits(repoOwner, repoName, opts)
	if err != nil {
		log.Println("ERROR: Failed to get commits from GitHub: ", err)
		environment.LatestGitHubCommit = ""
	} else {
		environment.LatestGitHubCommit = *commits[0].SHA
	}
	project.Environments[i] = environment
}

// retrieveCommits fetches the latest deployed commits as well
// as the latest GitHub commits for a given Project.
// it will also check if the user has permission to pull.
func retrieveCommits(r *http.Request, project goship.Project, deployUser string) (goship.Project, error) {
	// define a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup
	githubToken := os.Getenv(gitHubAPITokenEnvVar)
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	client := github.NewClient(oauth2.NewClient(oauth2.NoContext, ts))
	for i, environment := range project.Environments {
		for j, host := range environment.Hosts {
			// start a goroutine for SSHing on to the machine
			wg.Add(1)
			go getCommit(&wg, project, environment, host, deployUser, i, j)
		}
		wg.Add(1)
		go getLatestGitHubCommit(&wg, project, environment, client, project.RepoOwner, project.RepoName, i)
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
	u, err := getUser(r)
	if err != nil {
		log.Printf("Failed to get user %s", err)
	}
	return filterProject(project, r, u), err
}

// Will return true if the user has a team permission non read only
func userHasDeployPermission(g githublib.Client, owner, repo, user string) (pull bool, err error) {
	// List the  all the teams for a repository.
	teams, _, err := g.ListTeams(owner, repo, nil)
	if err != nil {
		fmt.Printf("Failure getting Organizations List %s", err)
	}
	// Iterate through the teams for a repo, if a user is a member of a non read-only team exit with false.
	pull = false
	for _, team := range teams {
		o, _, err := g.IsTeamMember(*team.ID, user)
		if err != nil {
			fmt.Printf("\nFailure getting Is Team Member from Org \n [%s]", err)
			return false, err
		}
		// if user is a member of a non read only team return false
		if o == true && *team.Permission != "pull" {
			return true, nil
		}

	}
	return pull, err
}

// Returns true if the github user is a current  "collaborator" on a project.  Used to allow the user to deploy the project.
func isCollaborator(owner, repo, user string) bool {

	if authentication.authorization != true {
		return true
	}

	g := newGithubClient()
	m, _, err := g.IsCollaborator(owner, repo, user)
	if err != nil {
		log.Printf("Failure getting Collaboration Status of User: %s %s %s err: %s", owner, user, repo, err)
		return false
	}
	return m
}
