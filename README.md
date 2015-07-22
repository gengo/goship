[![Build Status](https://travis-ci.org/gengo/goship.svg?branch=master)](https://travis-ci.org/gengo/goship)

# GoShip

A simple tool for deploying code to servers.

![pirate gopher](https://cloud.githubusercontent.com/assets/3772659/8693461/3c5f74a8-2b12-11e5-9a27-ff4421589df6.png)

### What it does:

GoShip SSHes into the machines that you list in ETCD and gets the latest revision from the specified git repository. It then compares that to the latest revision on GitHub and, if they differ, shows a link to the diff as well as a Deploy button. You can then deploy by clicking the button, and will show you the output of the deployment command, as well as save the output, diff, and whether the command succeeded.

![GoShip Index Page Screenshot](https://cloud.githubusercontent.com/assets/3772659/8693471/55ec2592-2b12-11e5-965f-8e572309c945.png)

### Usage

Export your GitHub API token:

    export GITHUB_API_TOKEN="your-organization-github-token-here"

### Github Omniauth Integration:

    Users who are collaborator on a repo can 'see' that repo in Goship.
    You must create a developer application to use omniauth.
    If you do NOT add the appropriate env keys below AUTH will be OFF I.E. Please be careful and check the logs.
    Please  note the "Authorization callback URL" should match your site i.e. "http://<your-url-and-port>/auth/github/callback"

    export GITHUB_RANDOM_HASH_KEY="some-random-hash-here";
    export GITHUB_OMNI_AUTH_ID="github-application-id";
    export GITHUB_OMNI_AUTH_KEY="github-application-key";
    export GITHUB_CALLBACK_URL="http://<your-url-and-port>";  // must match that given to Github! Would be 127.0.0.1:port for testing

    If authentication is 'turned on', organization 'team' members who are collaborators and exclusively on a 'pull' only team will be able to see a repo, however the deploy button will be diasbled for them.

Create an ETCD server / follow the instructions in the etcd README:

    https://github.com/coreos/etcd


There are various tools to update your ETCD server including the etcdctl client or curl, you can also use a variety of clients and JSON formatting:
There is also a convert.go in tools that can be used to 'bootstrap' etcd.


   #example setup using etcd
   https://github.com/coreos/etcdctl/

```
   etcdctl set /deploy_user 'deployer'
   finish deployment. e.g. Notify chat room
   etcdctl mkdir '/projects'
   etcdctl mkdir '/projects/my-project'
   etcdctl set /projects/my-project/repo_name 'my-project'
   etcdctl set /projects/my-project/repo_owner 'github-user'
   etcdctl set /projects/my-project/project_name 'My Project'
   etcdctl mkdir /projects/my-project/environments
   etcdctl mkdir /projects/my-project/environments/staging
   etcdctl mkdir /projects/my-project/environments/staging/hosts
   etcdctl mkdir /projects/my-project/environments/staging/hosts/myproject-staging-01.gengo.com
   etcdctl set /projects/my-project/environments/staging/branch master
   etcdctl set /projects/my-project/environments/staging/repo_path PATH_TO_REPOSITORY/.git
   etcdctl set /projects/my-project/environments/staging/deploy "/tmp/deploy -p=my-project -e=staging" # You need to set your deployment command here. This is an example using `tools/deploy/deploy.go`
```

   #curl example

```
   curl -L http://127.0.0.1:4001/projects/my-project/environments/staging/deploy -XPUT -d value="/path/to/deployscripts/myproj_staging.sh"
```

   #convert.go example in the tools folder of goship. ( config.yml and etcd settings are configurable - run with -h for options)

```
   go run convert.go -c /mnt/srv/http/gengo/goship/shared/config.yml
```

Then run the server manually

```shell
go run goship.go -b localhost:8888 -k ~/.ssh/id_rsa
```

Available command line flags for the `go run goship.go` command are:

```
 -b [bind address]             Address to bind (default localhost:8000)
 -k [id_rsa key]               Path to private SSH key for connecting to Github (default id_rsa)
 -d [data path]                Path to data directory (default ./data/)
 -e [etcd location]            Full URL to ETCD Server. Defaults to localhost
 -f [deploy confirmation flag] Flag to specify if user is prompted with confirmation dialog before deploys. Defaults to True
```

### Chat Notifications
To notify a chat room when the Deploy button is pushed, create a script that takes a message as an argument and sends the message to the room, and then add it to etcd:

```
etcdctl set /notify '/home/deployer/notify.sh'
```

[Sevabot](http://sevabot-skype-bot.readthedocs.org/en/latest/) is a good choice for Skype.

### Tools

There are some tools added in the /tools directory that can be used interface with Goship
1) convert.go: takes a config.yml  file and converts it to ETCD. Used for bootstrapping ETCD from the original
conf file.
2) deploy.go:  Can be used as a script by the "deploy" to create a knife solo command which reads in the appropriate servers from ETCD and runs knife solo.

### Plugins

Goship suffices as a basic application to aid your deployments. However, you may wish to extend Goship with some custom UI on its home page with plugins.

To do so, head over to [Plugins](plugins).

GoShip was inspired by [Rackspace's Dreadnot](https://github.com/racker/dreadnot) ([UI image](http://c179631.r31.cf0.rackcdn.com/dreadnot-overview.png)) and [Etsy's Deployinator](https://github.com/etsy/deployinator/) ([UI image](http://farm5.staticflickr.com/4065/4620552264_9e0fdf634d_b.jpg)).

The GoShip logo is an adaptation of the [Go gopher](http://blog.golang.org/gopher) created by Renee French under the [Creative Commons Attribution 3.0 license](https://creativecommons.org/licenses/by/3.0/).
