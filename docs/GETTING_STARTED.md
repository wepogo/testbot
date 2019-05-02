# Getting Started

testbot consists of two services:

* `testbot farmer` is a process that coordinates activity between GitHub
  and worker processes.
  See [`farmer/main.go`](farmer/main.go) for theory of operation.
* `testbot worker` is a process that runs the tests.
  See [`worker/main.go`](worker/main.go) for theory of operation.

## Create a GitHub user as your bot

TODO

## Deploy to Heroku

Clone this repo:

```
git clone https://github.com/wepogo/testbot
cd testbot
```

Create a Heroku app for testbot farmer.
Set it as the `farmer` Git remote:

```
export FARMER=your-heroku-app-name
git remote add farmer git@heroku.com:$FARMER.git
```

Create a Heroku app for your testbot workers.
Set it as the `workers` Git remote:

```
export WORKERS=your-heroku-app-name
git remote add workers git@heroku.com:$WORKERS.git
```

## Farmer

Initialize the schema:

```
psql `heroku config:get DATABASE_URL -a $FARMER` < ./farmer/schema.sql
```

Create a [GitHub personal access token](https://github.com/settings/tokens)
with `repo` and `write:repo_hook` scopes and set it:

```
heroku config:set GITHUB_TOKEN=YOUR_TOKEN -a $FARMER
```

Set the GitHub organization and repository names of the repo
you plan to test with testbot:

```
heroku config:set GITHUB_ORG=YOUR_ORG GITHUB_REPO=YOUR_REPO -a $FARMER
```

Create a GitHub OAuth2 application in the GitHub organization.
It is used for authenticating access to the farmer's web UI.
Set its client ID and client secret in the farmer's config:

```
heroku config:set CLIENT_ID=YOUR_CLIENT_ID CLIENT_SECRET=YOUR_CLIENT_SECRET -a $FARMER
```

Deploy testbot farmer:

```
git push farmer master
```

Don't scale the farmer app to more than 1 dyno.
If you do, live test output will become unreliable.
Everything else should continue to work fine, but
the live test output assumes there is only one farmer.

## Worker

All of your repo's dependencies must be set up on the worker host
in order for the worker processes to run all tests.

The worker can be configured with these environment variables:

```
GIT_CREDENTIALS
S3_REGION
S3_BUCKET
```
