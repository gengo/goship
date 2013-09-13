# GoShip

A simple tool for deploying code to servers.

GoShip was inspired by [Rackspace's Dreadnot](https://github.com/racker/dreadnot) ([UI image](http://c179631.r31.cf0.rackcdn.com/dreadnot-overview.png)) and [Etsy's Deployinator](https://github.com/etsy/deployinator/) ([UI image](http://farm5.staticflickr.com/4065/4620552264_9e0fdf634d_b.jpg)).

### Installation

    go get github.com/gengo/goship

### Usage

Export your GitHub API token:

    export GITHUB_API_TOKEN="your-organization-github-token-here"

Create a config.yml file:

```yaml
deploy_user: deployer
projects:
    - my_project:
        project_name: My Project
        repo_owner: github-user
        repo_name: my-project
        environments:
            - qa: 
                deploy: knife solo cook -i %s %s
                repo_path: /path/to/myproject/.git
                hosts:
                    - qa.myproject.com
                branch: sprint_branch
            - staging:
                deploy: knife solo cook -i %s %s
                repo_path: /path/to/myproject/.git
                hosts:
                    - staging.myproject.com
                branch: code_freeze
            - production:
                deploy: knife solo cook -i %s %s
                repo_path: /path/to/myproject/.git
                hosts:
                    - prod-01.myproject.com
                    - prod-02.myproject.com
                branch: master
```

Then run the server:

```shell
$GOPATH/bin/goship
```
