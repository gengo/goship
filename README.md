[![Build Status](https://travis-ci.org/gengo/goship.svg?branch=master)](https://travis-ci.org/gengo/goship)

# GoShip

A simple tool for deploying code to servers.

![pirate gopher](http://i.imgur.com/RLvkHka.png)

### What it does:

GoShip SSHes into the machines that you list in ETCD and gets the latest revision from the specified git repository. It then compares that to the latest revision on GitHub and, if they differ, shows a link to the diff as well as a Deploy button. You can then deploy by clicking the button, and will show you the output of the deployment command, as well as save the output, diff, and whether the command succeeded.

![GoShip Index Page Screenshot](http://tryimg.com/4/goshi.png)

### Usage

Export your GitHub API token:

    export GITHUB_API_TOKEN="your-organization-github-token-here"

Create an ETCD server / follow the instructions in the etcd README:

    https://github.com/coreos/etcd

Update your ETCD server via the etcctl client or curl:



   #example setup using etcd
   https://github.com/coreos/etcdctl/

```
   etcdctl set /deploy_user 'deployer'
   etcdctl mkdir 'projects'
   etcdctl mkdir 'projects/my-project'
   etcdctl set /projects/project_name 'My Project'
   etcdctl set /projects/repo_owner 'github-user'
   etcdctl set /projects/project_name 'My Project'
   
   etcdctl mkdir 'projects/my-project/environments'

   etcdctl mkdir 'projects/my-project/environments/staging'
   etcdctl set 'projects/my-project/environments/staging/deploy' '/path/to/deployscripts/myproj_staging.sh'
   etcdctl set 'projects/my-project/environments/staging/branch' 'code_freeze'
   etcdctl set 'projects/my-project/environments/staging/revision' '1234567'
   etcdctl set 'projects/my-project/environments/staging/repo_path' /path/to/myproject/.git
   etcdctl mkdir 'projects/my-project/environments/staging/hosts'
   etcdctl mkdir 'projects/my-project/environments/staging/hosts/staging.myproject.com'

   etcdctl mkdir 'projects/my-project/environments/production'
   etcdctl set 'projects/my-project/environments/production/deploy' '/path/to/deployscripts/myproj_live.sh'
   etcdctl set 'projects/my-project/environments/staging/repo_path' /path/to/myproject/.git
   etcdctl set 'projects/my-project/environments/staging/branch' 'master'
   etcdctl set 'projects/my-project/environments/staging/revision' 'head'
   etcdctl mkdir 'projects/my-project/environments/staging/hosts'
   etcdctl mkdir 'projects/my-project/environments/staging/hosts/prod-01.myproject.com'
   etcdctl mkdir 'projects/my-project/environments/staging/hosts/prod-02.myproject.com'
```

   #curl example
```
   curl -L http://127.0.0.1:4001/projects/my-project/environments/staging/deploy -XPUT -d value="/path/to/deployscripts/myproj_staging.sh"
```
   
Then run the server manually

```shell
go run goship.go -b localhost:8888 -k ~/.ssh/id_rsa
```

Available command line flags for the `go run goship.go` command are:

```
 -b [bind address]  Address to bind (default localhost:8000)
 -c [config file]   Config file (default ./config.yml)
 -k [id_rsa key]    Path to private SSH key for connecting to Github (default id_rsa)
 -d [data path]     Path to data directory (default ./data/)
```

### Chat Notifications
To notify a chat room when the Deploy button is pushed, create a script that takes a message as an argument and sends the message to the room, and then add it to the config like so:

```yaml
notify: ./notifications/notify.sh
```

[Sevabot](http://sevabot-skype-bot.readthedocs.org/en/latest/) is a good choice for Skype.

GoShip was inspired by [Rackspace's Dreadnot](https://github.com/racker/dreadnot) ([UI image](http://c179631.r31.cf0.rackcdn.com/dreadnot-overview.png)) and [Etsy's Deployinator](https://github.com/etsy/deployinator/) ([UI image](http://farm5.staticflickr.com/4065/4620552264_9e0fdf634d_b.jpg)).

The GoShip logo is an adaptation of the [Go gopher](http://blog.golang.org/gopher) created by Renee French under the [Creative Commons Attribution 3.0 license](https://creativecommons.org/licenses/by/3.0/).
