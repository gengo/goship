// #!/bin/bash
package main

// This script polls ETCD and builds a Chef deploy script.

import (
	"flag"
	"log"
	"os/exec"
	"strings"

	"github.com/coreos/go-etcd/etcd"
	"github.com/gengo/goship/lib"
)

// Variables
var (
	knifePath  = flag.String("s", ".chef/knife.rb", "KnifePath (.chef/knife.rb)")
	chefPath   = flag.String("c", "/srv/http/gengo/devops-tools/daidokoro", "Chef Path (required)")
	pemKey     = flag.String("k", "/home/deployer/.ssh/dszydlowski.pem", "PEM Key (default /home/deployer/.ssh/dszydlowski.pem)")
	deployUser = flag.String("u", "ubuntu", "deploy user (default /ubuntu)")
	deployProj = flag.String("p", "", "project (required)")
	deployEnv  = flag.String("e", "", "environment (required)")
)

func execCmd(cmd string) {
	parts := strings.Fields(cmd)
	head := parts[0]
	parts = parts[1:len(parts)]
	out, err := exec.Command(head, parts...).Output()
	if err != nil {
		log.Fatal("Command failed to run: ", err)
	}
	log.Print(out)
}

func main() {
	flag.Parse()
	c, err := goship.ParseETCD(etcd.NewClient([]string{"http://127.0.0.1:4001"}))
	if err != nil {
		log.Fatalf("Error Parsing ETCD: %s", err)
	}
	projectEnv, err := goship.GetEnvironmentFromName(c.Projects, *deployProj, *deployEnv)
	if err != nil {
		log.Fatalf("Error Getting Project %s %s %s", *deployProj, *deployEnv, err)
	}
	log.Printf("Project Name: %s Environment Name: %s", *deployEnv, projectEnv.Name)
	servers := projectEnv.Hosts
	for _, h := range servers {
		d := "cd " + *chefPath + " && knife solo cook -c " + *knifePath + " -i " + *pemKey + " " + *deployUser + "@" + h.URI
		log.Printf("Preparing Knife command: %s", d)
		execCmd(d)
	}
}
