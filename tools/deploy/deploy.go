package main

// This script polls ETCD and executes Chef knife solo cook.

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	gsconfig "github.com/gengo/goship/lib/config"
	"github.com/golang/glog"
	"github.com/kylelemons/go-gypsy/yaml"
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
	chefRepo   string
	chefPath   string
	knifePath  string
	pemKey     string
	deployUser string
	etcdServer string
}

func checkMissingConf(s, v, f string) {
	if len(s) < 1 {
		glog.Fatalf("Warning: Missing %s in config file [%s]", v, f)
	}
}

func parseConfig() (c config) {
	config, err := yaml.ReadFile(*configFile)
	if err != nil {
		glog.Fatalf("Fatal: Can't parse conf file %s", *configFile)
	}
	c.chefRepo, err = config.Get("chef_repo")
	checkMissingConf(c.chefRepo, "chef_repo", *configFile)
	c.chefPath, err = config.Get("chef_path")
	checkMissingConf(c.chefPath, "chef_path", *configFile)
	c.knifePath, err = config.Get("knife_path")
	checkMissingConf(c.knifePath, "knife_path", *configFile)
	c.pemKey, err = config.Get("pem_key")
	checkMissingConf(c.pemKey, "pem_key", *configFile)
	c.deployUser, err = config.Get("deploy_user")
	checkMissingConf(c.deployUser, "deploy_user", *configFile)
	c.etcdServer, err = config.Get("etcd_server")
	if len(c.etcdServer) < 1 {
		c.etcdServer = "http://127.0.0.1:4001"
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
	// TODO: refactor "execCmd" and run commands at once
	gitPullCmd := "/usr/bin/git --git-dir=" + conf.chefRepo + "/.git --work-tree=" + conf.chefRepo + " pull origin " + *deployToolBranch
	_, err := execCmd(gitPullCmd, conf)
	if err != nil {
		glog.Fatal("Failed to pull: ", err)
	}
	gitCheckoutCmd := "/usr/bin/git --git-dir=" + conf.chefRepo + "/.git --work-tree=" + conf.chefRepo + " checkout " + *deployToolBranch
	_, err = execCmd(gitCheckoutCmd, conf)
	if err != nil {
		glog.Fatal("Failed to checkout: ", err)
	}
	glog.Infof("Updated devops-tools to the latest %s branch", *deployToolBranch)
}

func execCmd(icmd string, conf config) (output string, err error) {
	os.Chdir(conf.chefPath)

	parts := strings.Fields(icmd)
	head := parts[0]
	parts = parts[1:len(parts)]

	cmd := exec.Command(head, parts...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		glog.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		glog.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		glog.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		o := scanner.Text()
		output += o
		fmt.Println(o)

	}
	if err := scanner.Err(); err != nil {
		glog.Errorf("Error reading standard output stream: %s", err)
	}
	scanner = bufio.NewScanner(stderr)
	for scanner.Scan() {
		glog.Infof(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		glog.Errorf("Error reading standard error stream: %s", err)
	}
	if err := cmd.Wait(); err != nil {
		glog.Fatalf("Error waiting for Chef to complete %s", err)
	}
	return output, err
}

func main() {
	flag.Parse()
	conf := parseConfig()
	if *skipUpdate == false {
		updateChefRepo(conf)
	}
	if *pullOnly == false {
		c, err := gsconfig.Load(etcd.NewClient([]string{conf.etcdServer}))
		if err != nil {
			glog.Fatalf("Error parsing ETCD: %s", err)
		}
		projectEnv, err := gsconfig.EnvironmentFromName(c.Projects, *deployProj, *deployEnv)
		if err != nil {
			glog.Fatalf("Error getting project %s %s %s", *deployProj, *deployEnv, err)
		}
		glog.Infof("Deploying project name: %s environment Name: %s", *deployEnv, projectEnv.Name)
		servers := projectEnv.Hosts
		var d, e string
		if *chefRunlist != "" {
			e = " -o \"" + *chefRunlist + "\" "
		}
		for _, h := range servers {
			if *bootstrap == true {
				d = "knife solo bootstrap -c " + conf.knifePath + " -i " + conf.pemKey + " --no-host-key-verify " + e + conf.deployUser + "@" + h.URI
			} else {
				d = "knife solo cook -c " + conf.knifePath + " -i " + conf.pemKey + " --no-host-key-verify " + e + conf.deployUser + "@" + h.URI
			}
			glog.Infof("Deploying to server: %s", h.URI)
			glog.Infof("Preparing Knife command: %s", d)
			_, err := execCmd(d, conf)
			if err != nil {
				glog.Fatalf("Error Executing command %s", err)
			}
		}
	}

}
