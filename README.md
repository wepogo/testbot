# testbot

testbot is our CI tool. It consists of two services.

`testbot farmer` is a process that coordinates activity between GitHub
and worker processes.
See [`farmer/main.go`](farmer/main.go) for theory of operation.

`testbot worker` is a process that runs the tests.
See [`worker/main.go`](worker/main.go) for theory of operation.

## Developer Setup

### 1. Install Binaries

```sh
go install github.com/wepogo/testbot/cmd/testbot
```

### 2. Create Database

```sh
createdb testbot
psql testbot < $I10R/testbot/farmer/schema.sql
```

### 3. Start Ngrok

In a new shell, start an [ngrok](https://ngrok.com) tunnel:

```sh
ngrok http 1994
```

### 4. Create GitHub credential

Create a [GitHub personal access token](https://github.com/settings/tokens)
with `repo` and `write:repo_hook` scopes.

### 5. Start Farmer Process

In another shell:

```sh
BASE_URL=https://{{ THE_NGROK_TUNNEL_YOU_CREATED }}.ngrok.io/ \
DATABASE_URL=postgres:///testbot?sslmode=disable \
GITHUB_TOKEN={{ THE_TOKEN_YOU_CREATED }} \
GITHUB_REPO=citest \
GITHUB_ORG=wepogo \
HOOK_SECRET=anything \
testbot farmer
```

### 6. Start Worker Process

In another shell:

```sh
FARMER_URL=http://localhost:1994 \
GITHUB_REPO=citest \
testbot worker
```

Make a change to [citest](https://github.com/wepogo/citest) in a branch and
open a pull request. You should receive the pull request hook to your local
farmer and see your local worker pick up the job.

When you're done, you may need to clean up the automatically-created webhook
in [citest's webhook settings](https://github.com/wepogo/citest/settings/hooks).

## The Worker Host

The worker has unique host requirements. We need all of the dependencies
to be setup on the host so that we can run all of our code's tests.

The host is setup using this script: `$I10R/infra/setup.sh`

### Making Changes on a Live Host

Changes to `testbot worker` can also be edited and tested on a live box:

```
aws-vault exec i10r -- chssh ortest $n
cd testbot-worker/src/i10r.io
git fetch origin && git checkout $SHA
vim testbot/worker
GOPATH=/home/ubuntu/testbot-worker go install -tags aws chain/cmd/testbot
~/testbot-worker/bin/testbot onejob $SHA $DIR $NAME
```

## Deploying to Heroku

### Farmer

Initialize the schema.

```
export APP=testbot-farmer
psql `heroku config:get DATABASE_URL -a $APP` < ./farmer/schema.sql
```

```
git remote add ci git@heroku.com:$APP.git
git push ci main:master
```

#### Logs

```
heroku logs --tail -r ci
```

#### PSQL

```
heroku pg:psql -r ci
```

Note: don't scale the farmer app to more than 1 dyno.
If you do, live test output will become unreliable.
(Everything else should continue to work fine, but
the live test output assumes there is only one farmer
process. It's not dangerous to run multiple farmer
processes, it will just make live output flaky.)

### Worker

The workers are currently deployed to the ortest stack.
The stacks terraform is in: `$I10R/infra/terraform/testbot-worker`

#### Launching an Instance

Launch the instance:

```
aws-vault exec i10r -- launch-instance ortest testbot-worker

Creating 4 in stack ortest
Subnet ID: subnet-8b4799f2
VPC ID: vpc-f0264c89
Security Group ID: sg-b4ab93cb
AMI: ami-f8c75580
waiting for 4 (i-03933d97cbacf46b2) to provision
```

Configure it:

```
aws-vault exec i10r -- chconfec2 ortest $n
```

#### Deploying an Instance

Deploy `git rev-parse HEAD` to start testbot on the new box.

```
aws-vault exec i10r -- chdeploy ortest testbot-worker $n $sha
```

#### Environment

`S3_BUCKET`, `GIT_CREDENTIALS`, and `NETLIFY_AUTH_TOKEN`
are stored in Parameter Store.
See the `NETLIFY_AUTH_TOKEN` commit for how to add an environment variable.
