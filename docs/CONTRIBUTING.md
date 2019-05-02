# Contributing

Install binaries:

```
go install github.com/wepogo/testbot/cmd/testbot
```

Create database:

```sh
createdb testbot
psql testbot < $I10R/testbot/farmer/schema.sql
```

In a new shell, start an [ngrok](https://ngrok.com) tunnel:

```sh
ngrok http 1994
```

Create a [GitHub personal access token](https://github.com/settings/tokens)
with `repo` and `write:repo_hook` scopes.

In another shell:

```
BASE_URL=https://{{ YOUR_NGROK_TUNNEL }}.ngrok.io/ \
DATABASE_URL=postgres:///testbot?sslmode=disable \
GITHUB_TOKEN={{ YOUR_TOKEN }} \
GITHUB_ORG=wepogo \
GITHUB_REPO=citest \
HOOK_SECRET=anything \
testbot farmer
```

In another shell, start a worker process:

```sh
FARMER_URL=http://localhost:1994 \
GITHUB_ORG=wepogo \
GITHUB_REPO=citest \
testbot worker
```

Make a change to [citest](https://github.com/wepogo/citest) in a branch and
open a pull request. You should receive the pull request hook to your local
farmer and see your local worker pick up the job.

When you're done, you may need to clean up the automatically-created webhook
in [citest's webhook settings](https://github.com/wepogo/citest/settings/hooks).
