# GoShip

A simple tool for deploying code to servers.

GoShip was inspired by [Rackspace's Dreadnot](https://github.com/racker/dreadnot) ([UI image](http://c179631.r31.cf0.rackcdn.com/dreadnot-overview.png)) and [Etsy's Deployinator](https://github.com/etsy/deployinator/) ([UI image](http://farm5.staticflickr.com/4065/4620552264_9e0fdf634d_b.jpg)).

### Installation

    go get github.com/gengo/goship
    go install github.com/gengo/goship

### Usage

Export your GitHub API token:

    export GITHUB_API_TOKEN="your-organization-github-token-here"

Create a config.yml file:

```yaml
# The user that will SSH into the servers to get the latest git revisions
deploy_user: deployer
# Domain you'll host goship on
goship_host: goship.yourdomain.com
# used for getting open pull requests across all repos in an org
orgs:
    - myOrg
projects:
    - my_project:
        project_name: My Project
        repo_owner: github-user
        repo_name: my-project
        environments:
            - qa:
                deploy: /path/to/deployscripts/myproj_qa.sh
                repo_path: /path/to/myproject/.git
                hosts:
                    - qa.myproject.com
                branch: sprint_branch
            - staging:
                deploy: /path/to/deployscripts/myproj_staging.sh
                repo_path: /path/to/myproject/.git
                hosts:
                    - staging.myproject.com
                branch: code_freeze
            - production:
                deploy: /path/to/deployscripts/myproj_live.sh
                repo_path: /path/to/myproject/.git
                hosts:
                    - prod-01.myproject.com
                    - prod-02.myproject.com
                branch: master
```

Then run the server manually

```shell
go run goship.go -p 8888 -k ~/.ssh/id_rsa
```

or from the script:

```shell
$GOPATH/bin/goship
```

Available command line flags for the `go run goship.go` command are:

```
 -p [port]          Port number (default 8000)
 -c [config file]   Config file (default ./config.yml)
 -k [id_rsa key]    Path to private SSH key for connecting to Github (default id_rsa)
```
