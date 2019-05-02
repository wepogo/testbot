# testbot

testbot is an open source, run-it-yourself Continuous Integration (CI) service.

It consists of two services:

* `testbot farmer` is a process that coordinates activity between GitHub
  and worker processes.
  See [`farmer/main.go`](farmer/main.go) for theory of operation.
* `testbot worker` is a process that runs the tests.
  See [`worker/main.go`](worker/main.go) for theory of operation.

## Deploying to Heroku

Clone this repo:

```
git clone https://github.com/wepogo/testbot
cd testbot
```

## Farmer

Set a local `$APP` environment variable to your Heroku app name:

```
export APP=testbot-farmer
```

Initialize the schema:

```
psql `heroku config:get DATABASE_URL -a $APP` < ./farmer/schema.sql
```

Create a [GitHub personal access token](https://github.com/settings/tokens)
with `repo` and `write:repo_hook` scopes and set it:

```
heroku config:set GITHUB_TOKEN=YOUR_TOKEN -a $APP
```

Set the GitHub organization and repository names of the repo
you plan to test with testbot:

```
heroku config:set GITHUB_ORG=YOUR_ORG GITHUB_REPO=YOUR_REPO -a $APP
```

Deploy testbot:

```
git remote add ci git@heroku.com:$APP.git
git push ci master
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
