# Contributing

Install binary:

```
go install ./cmd/testbot
```

Create database:

```sh
createdb testbot
psql testbot < ./farmer/schema.sql
```

In a new shell, start an [ngrok](https://ngrok.com) tunnel:

```sh
ngrok http 1994
```

Create a [GitHub personal access token](https://github.com/settings/tokens)
with `read:org`, `repo`, and `write:repo_hook` scopes.

In another shell:

```
DATABASE_URL=postgres:///testbot?sslmode=disable \
FARMER_URL=https://changeme.ngrok.io/ \
GITHUB_ORG=wepogo \
GITHUB_REPO=citest \
GITHUB_TOKEN=changeme \
HOOK_SECRET=changeme \
testbot farmer
```

In another shell, start a worker process:

```sh
FARMER_URL=http://localhost:1994 \
GITHUB_ORG=wepogo \
GITHUB_REPO=citest \
S3_BUCKET=pogo-testbot-logs \
S3_REGION=us-west-2 \
testbot worker
```

Make a change to [citest](https://github.com/wepogo/citest) in a branch and
open a pull request. You should receive the pull request hook to your local
farmer and see your local worker pick up the job.

When you're done, you may need to clean up the automatically-created webhook
in [citest's webhook settings](https://github.com/wepogo/citest/settings/hooks).
