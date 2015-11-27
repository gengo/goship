// Command deploy is a Gengo-specific deployment script which can be invoked by Goship.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	gsconfig "github.com/gengo/goship/lib/config"
	"github.com/golang/glog"
	yaml "gopkg.in/yaml.v2"
)

var (
	deployProj       = flag.String("p", "", "project (required)")
	deployEnv        = flag.String("e", "", "environment (required)")
	configFile       = flag.String("g", "/tmp/deploy.yaml", "shared config setting ( default /tmp/deploy.yaml)")
	deployToolBranch = flag.String("d", "master", "deploy tool branch ( default master)")
	pullOnly         = flag.Bool("o", false, "chef update only (default false)")
	chefRunlist      = flag.String("r", "", "custom run-list for Chef (default none)")
	skipUpdate       = flag.Bool("m", false, "skip the chef update (default false)")
	bootstrap        = flag.Bool("b", false, "bootstrap a server ( default false)")
)

// gitHubPaginationLimit is the default pagination limit for requests to the GitHub API that return multiple items.
const (
	gitHubPaginationLimit = 30
	gitHubAPITokenEnvVar  = "GITHUB_API_TOKEN"
)

// config contains the information from config.yml.
type config struct {
	ChefRepo   string `yaml:"chef_repo,omitempty"`
	ChefPath   string `yaml:"chef_path,omitempty"`
	KnifePath  string `yaml:"knife_path,omitempty"`
	PemKey     string `yaml:"pem_key,omitempty"`
	DeployUser string `yaml:"deploy_user,omitempty"`
	EtcdServer string `yaml:"etcd_server,omitempty"`
}

func parseConfig() config {
	buf, err := ioutil.ReadFile(*configFile)
	if err != nil {
		glog.Fatalf("Can't open conf file %s: %v", *configFile, err)
	}
	var c config
	if err := yaml.Unmarshal(buf, &c); err != nil {
		glog.Fatalf("Can't parse conf file %s: %v", *configFile, err)
	}
	for _, item := range []struct {
		attr, value string
	}{
		{"chef_repo", c.ChefRepo},
		{"chef_path", c.ChefPath},
		{"knife_path", c.KnifePath},
		{"pem_key", c.PemKey},
		{"deploy_user", c.DeployUser},
	} {
		if item.value == "" {
			glog.Fatalf("configuration %s is missing in %s", item.attr, *configFile)
		}
	}
	if c.EtcdServer == "" {
		c.EtcdServer = "http://127.0.0.1:4001"
	}
	return c
}

// Update ChefRepo to ensure the latest chef cookbooks are pulled before deploying.
// Current implementation is Gengo specified. Please re-implement this function according to your environment
func updateChefRepo(conf config) {
	glog.Infof("Updating devops-tools")
	os.Setenv("GIT_SSH", "/tmp/private_code/wrap-ssh4git.sh")
	os.Setenv("EMAIL", "devops@gengo.com")
	os.Setenv("NAME", "gengodev")
	gitBase := []string{
		"/usr/bin/git",
		"--git-dir", filepath.Join(conf.ChefRepo, ".git"),
		"--work-tree", conf.ChefRepo,
	}
	gitPullCmd := append(gitBase, "pull", "origin", *deployToolBranch)
	_, err := execCmd(gitPullCmd, conf)
	if err != nil {
		glog.Fatal("Failed to pull: ", err)
	}
	gitCheckoutCmd := append(gitBase, "checkout", *deployToolBranch)
	_, err = execCmd(gitCheckoutCmd, conf)
	if err != nil {
		glog.Fatal("Failed to checkout: ", err)
	}
	_, err = os.Stat(filepath.Join(conf.ChefRepo, "Berksfile"))
	if !os.IsNotExist(err) {
		if err != nil {
			glog.Fatal("Failed to access to Berksfile: %v", err)
		}
		_, err = execCmd([]string{"berks", "--path", "cookbooks"}, conf)
		if err != nil {
			glog.Fatal("Failed to fetch third-party cookbooks: ", err)
		}
	}
	glog.Infof("Updated devops-tools to the latest %s branch", *deployToolBranch)
}

func execCmd(argv []string, conf config) (string, error) {
	var output bytes.Buffer

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = conf.ChefPath
	cmd.Stdout = io.MultiWriter(&output, os.Stdout)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		glog.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			glog.Infof(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			glog.Errorf("Error reading standard error stream: %s", err)
		}
	}()
	if err := cmd.Start(); err != nil {
		glog.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		glog.Fatalf("Error waiting for Chef to complete %s", err)
	}
	<-done
	return output.String(), err
}

func main() {
	flag.Parse()
	defer glog.Flush()

	conf := parseConfig()
	if !*skipUpdate {
		updateChefRepo(conf)
	}
	if !*pullOnly {
		c, err := gsconfig.Load(etcd.NewClient([]string{conf.EtcdServer}))
		if err != nil {
			glog.Fatalf("Error parsing ETCD: %s", err)
		}
		projectEnv, err := gsconfig.EnvironmentFromName(c.Projects, *deployProj, *deployEnv)
		if err != nil {
			glog.Fatalf("Error getting project %s %s %s", *deployProj, *deployEnv, err)
		}
		glog.Infof("Deploying project name: %s environment Name: %s", *deployEnv, projectEnv.Name)
		for _, h := range projectEnv.Hosts {
			var cmd []string
			if *bootstrap {
				cmd = []string{
					"knife", "solo", "bootstrap",
					"-c", conf.KnifePath,
					"-i", conf.PemKey,
					"--no-host-key-verify",
				}
			} else {
				cmd = []string{
					"knife", "solo", "cook",
					"-c", conf.KnifePath,
					"-i", conf.PemKey,
					"--no-host-key-verify",
				}
			}
			if *chefRunlist != "" {
				cmd = append(cmd, "-o", *chefRunlist)
			}
			cmd = append(cmd, fmt.Sprintf("%s@%s", conf.DeployUser, h))
			glog.Infof("Deploying to server: %s", h)
			glog.Infof("Preparing Knife command: %s", strings.Join(cmd, ""))
			_, err := execCmd(cmd, conf)
			if err != nil {
				glog.Fatalf("Error Executing command %s", err)
			}
		}
	}
}
