# Getting Started

testbot consists of two services:

* `testbot farmer` is a process that coordinates activity between GitHub
  and worker processes.
  See [`farmer/main.go`](farmer/main.go) for theory of operation.
* `testbot worker` is a process that runs the tests.
  See [`worker/main.go`](worker/main.go) for theory of operation.

## Create a GitHub user as your bot

TODO

## Initialize Heroku apps

Clone this repo:

```
git clone https://github.com/wepogo/testbot
cd testbot
```

Create a Heroku app for testbot farmer
and set it as the `farmer` Git remote:

```
heroku create --remote farmer --buildpack heroku/go
```

Create a Heroku app for testbot workers,
set it as the `workers` Git remote,
and add required buildpacks:

```
heroku create --remote workers --buildpack heroku/go
heroku buildpacks:add \
  --remote workers \
  --index 1 https://github.com/heroku/heroku-buildpack-ci-postgresql
heroku buildpacks:add \
  --remote workers \
  --index 2 https://www.github.com/jbowens/test-buildpack
```

Configure Heroku to compile testbot at build time
in order to be executed at Heroku run time:

```
heroku config:set GO_INSTALL_PACKAGE_SPEC=./cmd/testbot -r farmers
heroku config:set GO_INSTALL_PACKAGE_SPEC=./cmd/testbot -r workers
```

Set the GitHub organization and repository names of the repo
you plan to test with testbot:

```
heroku config:set GITHUB_ORG=changeme GITHUB_REPO=changeme -r farmer
heroku config:set GITHUB_ORG=changeme GITHUB_REPO=changeme -r workers
```

## Configure and deploy farmer

Create and initialize Postgres database:

```
heroku addons:create heroku-postgresql:hobby-dev -r farmer
psql `heroku config:get DATABASE_URL -r farmer` < ./farmer/schema.sql
```

Create a [GitHub personal access token](https://github.com/settings/tokens)
with `repo` and `write:repo_hook` scopes and set it:

```
heroku config:set GITHUB_TOKEN=changeme -r farmer
```

Create a GitHub OAuth2 app in the GitHub org of the repo to be tested.
It is used for authenticating access to the farmer's web UI.
Set its client ID and client secret in the farmer's config:

```
heroku config:set CLIENT_ID=changeme CLIENT_SECRET=changeme -r farmer
```

Deploy:

```
git push farmer master
```

Don't scale the farmer app to more than 1 dyno.
If you do, live test output will become unreliable.
Everything else should continue to work fine, but
the live test output assumes there is only one farmer.

## Configure and deploy workers

All of your repo's dependencies must be set up on the worker host
in order for the worker processes to run all tests.

The worker can be configured with these environment variables:

```
GIT_CREDENTIALS
S3_REGION
S3_BUCKET
```

Deploy:

```
git push workers master
```

Scale as many workers as you'd like:

```
heroku ps:scale workers=5 -r workers
```
