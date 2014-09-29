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

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib"
)

var (
	knifePath  = flag.String("s", ".chef/knife.rb", "KnifePath (.chef/knife.rb)")
	chefPath   = flag.String("c", "/srv/http/gengo/devops-tools/daidokoro", "Chef Path (required)")
	pemKey     = flag.String("k", "/home/deployer/.ssh/dszydlowski.pem", "PEM Key (default /home/deployer/.ssh/dszydlowski.pem)")
	deployUser = flag.String("u", "ubuntu", "deploy user (default /ubuntu)")
	deployProj = flag.String("p", "", "project (required)")
	deployEnv  = flag.String("e", "", "environment (required)")
)

func execCmd(icmd string) {
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
		fmt.Println(scanner.Text())
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
}

func main() {
	flag.Parse()
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
		d := "knife solo cook -c " + *knifePath + " -i " + *pemKey + " " + *deployUser + "@" + h.URI
		log.Printf("Deploying to server: %s", h.URI)
		log.Printf("Preparing Knife command: %s", d)
		execCmd(d)
	}
}
