# Getting Started

testbot consists of two services:

* `testbot farmer` is a process that coordinates activity between GitHub
  and worker processes.
  See [`farmer/main.go`](farmer/main.go) for theory of operation.
* `testbot worker` is a process that runs the tests.
  See [`worker/main.go`](worker/main.go) for theory of operation.

## Set up a GitHub account for your bot

[Create a GitHub account](https://github.com/) for your bot.
For example, [@testbot](https://github.com/testbot) is ours.

Add the GitHub user as an admin collaborator
on the GitHub repo you want testbot to test.

Accept the invitation to collaborate on the repo.

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

Create a Heroku app for testbot workers and
set it as the `workers` Git remote:

```
heroku create --remote workers
```

Set farmer URL to the newly created Heroku URL
in order for the services to communicate with each other:

```
heroku config:set FARMER_URL=https://changeme.herokuapp.com -r farmer
heroku config:set FARMER_URL=https://changeme.herokuapp.com -r workers
```

Set the GitHub organization and repository names of the repo
you plan to test with testbot:

```
heroku config:set GITHUB_ORG=changeme GITHUB_REPO=changeme -r farmer
heroku config:set GITHUB_ORG=changeme GITHUB_REPO=changeme -r workers
```

## Configure and deploy farmer

Configure Heroku to compile testbot:

```
heroku config:set GO_INSTALL_PACKAGE_SPEC=./cmd/testbot -r farmers
```

Create and initialize Postgres database:

```
heroku addons:create heroku-postgresql:hobby-dev -r farmer
psql `heroku config:get DATABASE_URL -r farmer` < ./farmer/schema.sql
```

Under your bot's GitHub account,
create a [GitHub personal access token](https://github.com/settings/tokens)
with `repo`, `read:org`, and `write:repo_hook` scopes.
Add it to the Heroku environment:

```
heroku config:set GITHUB_TOKEN=changeme -r farmer
```

Under your bot's GitHub account, create a
[GitHub OAuth application](https://github.com/settings/applications/new).
It is used for authenticating access to the farmer's web UI.
Set its "Homepage URL" and "Authorization callback URL" the farmer URL
(e.g. `https://changeme.herokuapp.com/`).
Add it to the Heroku environment:

```
heroku config:set CLIENT_ID=changeme CLIENT_SECRET=changeme -r farmer
```

Create a secret for creating GitHub hooks:

```
heroku config:set HOOK_SECRET=changeme -r farmer
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

Create a `Dockerfile.testbot` in your repo under test.
Deploy it to Heroku:

```
heroku container:push testbot --recursive -r workers
heroku container:release testbot -r workers
```

Under your bot's GitHub account,
create a [GitHub personal access token](https://github.com/settings/tokens)
with `repo`, `read:gpg_key`, `read:public_key`, `read:user` scopes.
Add it to the Heroku environment:

```
heroku config:set GIT_CREDENTIALS=https://botname:token@github.com -r workers
```

Set S3 bucket and region:

```
heroku config:set \
    AWS_REGION=changeme \
    AWS_ACCESS_KEY_ID=changeme \
    AWS_SECRET_ACCESS_KEY=changeme \
    S3_BUCKET=changeme \
-r workers
```

By default, jobs time out after 60 seconds.
To increase the timeout, set this config var to a valid
[duration string](https://golang.org/pkg/time/#ParseDuration):

```
heroku config:set JOB_TIMEOUT=5m -r workers
```

Deploy:

```
git push workers master
```

Scale as many workers as you'd like:

```
heroku ps:scale workers=5 -r workers
```

## Open a test pull request

Add a `Testfile` to any directory in your repo under test:

```
test1: echo "hello"
test2: echo "world"
```

Commit the change.
Open a pull request.
The tests should run immediately.

A GitHub "status check" should appear for each test job.
The status check name will be the directory name (`/` for root)
and test job name (`test1`, `test2`).

Click the "Details" link for one of the status checks.
A GitHub OAuth authorization screen should appear.

The app will not have been granted access to your organization yet.
Click "Grant" next to the organization name for the repo under test.
An email will be sent to the GitHub org's administrators.
Click the link in the email to grant access to the app.

Now, any time a user within in your organization
clicks "Details" and authorizes the OAuth app,
they will be able to see test results output.

## Optional: add branch protection rule

In your GitHub repo under test,
click "Settings > Branches > Add rule".
Use a "Branch name pattern" equal to your default branch (e.g. "master").

Click "Require status checks to pass before merging".
As you add real test jobs to your repo,
their names will appear here.
If you have a test job in the `Testfile` in the root of your repo,
it will run on every pull request.
You can mark is as "Required" here.
