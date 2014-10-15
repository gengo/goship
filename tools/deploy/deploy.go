package main

// This script polls ETCD and builds a Chef deploy script.

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"code.google.com/p/goauth2/oauth"
	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib"
	"github.com/google/go-github/github"
)

var (
	knifePath  = flag.String("s", ".chef/knife.rb", "KnifePath (.chef/knife.rb)")
	chefRepo   = flag.String("r", "/srv/http/gengo/devops-tools/", "Chef Repo (/srv/http/gengo/devops-tools/)")
	chefPath   = flag.String("c", "/srv/http/gengo/devops-tools/daidokoro", "Chef Path (/srv/http/gengo/devops-tools/daidokoro)")
	pemKey     = flag.String("k", "/home/deployer/.ssh/dszydlowski.pem", "PEM Key (default /home/deployer/.ssh/dszydlowski.pem)")
	deployUser = flag.String("u", "ubuntu", "deploy user (default /ubuntu)")
	deployProj = flag.String("p", "", "project (required)")
	deployEnv  = flag.String("e", "", "environment (required)")
	pullOnly   = flag.Bool("o", false, "chef update only (default false)")
	skipUpdate = flag.Bool("m", false, "skip the chef update (default false)")
)

// gitHubPaginationLimit is the default pagination limit for requests to the GitHub API that return multiple items.
const (
	gitHubPaginationLimit = 30
	gitHubAPITokenEnvVar  = "GITHUB_API_TOKEN"
)

// updateChefRepo ensures the lates chef cookbooks are pulled before deploying.
// Checks github first and ignores pull if already up to date.
func updateChefRepo(deployUser string) {
	githubToken := os.Getenv(gitHubAPITokenEnvVar)
	t := &oauth.Transport{
		Token: &oauth.Token{AccessToken: githubToken},
	}
	client := github.NewClient(t.Client())
	s := "git --git-dir=" + *chefRepo + "/.git rev-parse HEAD"
	localHash, _ := execCmd(s)
	commits, _, err := client.Repositories.ListCommits("Gengo", "devops-tools", nil)
	if err != nil {
		log.Fatal("ERROR:  failed to get commits from GitHub: Please try again later ", err)
	}
	remoteHash := *commits[0].SHA
	if localHash == remoteHash {
		log.Printf("Local Chef is up to date: Skipping Sync")
	} else {
		log.Printf("Chef is not up to date: \n %s does not equal %s", localHash, remoteHash)
		log.Println("Updating devops-tools")
		s := "git --git-dir=" + *chefRepo + "/.git pull origin master"
		_, err := execCmd(s)
		if err != nil {
			log.Fatal("ERROR:  Failed to pull latest devops_tools: ", err)
		}
		log.Println("Devops Tools Updated")
	}
}

func execCmd(icmd string) (output string, err error) {
	os.Chdir(*chefPath)

	parts := strings.Fields(icmd)
	head := parts[0]
	parts = parts[1:len(parts)]

	cmd := exec.Command(head, parts...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		o := scanner.Text()
		output += o
		fmt.Println(o)

	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading standard output stream: %s", err)
	}
	scanner = bufio.NewScanner(stderr)
	for scanner.Scan() {
		log.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading standard error stream: %s", err)
	}
	if err := cmd.Wait(); err != nil {
		log.Fatalf("Error waiting for Chef to complete %s", err)
	}
	return output, err
}

func main() {
	flag.Parse()
	if *skipUpdate == false {
		updateChefRepo(*deployUser)
	}
	if *pullOnly == false {
		c, err := goship.ParseETCD(etcd.NewClient([]string{"http://127.0.0.1:4001"}))
		if err != nil {
			log.Fatalf("Error parsing ETCD: %s", err)
		}
		projectEnv, err := goship.EnvironmentFromName(c.Projects, *deployProj, *deployEnv)
		if err != nil {
			log.Fatalf("Error getting project %s %s %s", *deployProj, *deployEnv, err)
		}
		log.Printf("Deploying project name: %s environment Name: %s", *deployEnv, projectEnv.Name)
		servers := projectEnv.Hosts
		for _, h := range servers {
			d := "knife solo cook -c " + *knifePath + " -i " + *pemKey + " --no-host-key-verify " + *deployUser + "@" + h.URI
			log.Printf("Deploying to server: %s", h.URI)
			log.Printf("Preparing Knife command: %s", d)
			_, err := execCmd(d)
			if err != nil {
				log.Fatalf("Error Executing command %s", err)
			}
		}
	}

}
