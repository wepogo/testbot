# testbot

testbot is a free,
open source,
run-it-yourself
Continuous Integration (CI) tool.

To run a testbot instance,
read [Getting Started](docs/GETTING_STARTED.md).

## Why

Try testbot if you want these things from your CI tool:

1. begin running the tests in < 5s after opening or editing a pull request
2. run only the tests relevant to the change
3. avoid leaking code, credentials, or data to a third-party CI provider

Common causes of slow-starting tests on hosted CI services
are multi-tenant queues and containerization.
In those environments,
noisy neighbors backlog the queues
and containers have cache misses.

If you organize your code into a [monorepo](https://www.statusok.com/monorepo),
your pull requests often will not need to run most tests.

## How it works

Imagine a monorepo that contains some files like this:

```
.
├── ...
├── dashboard
│   └── Testfile
├── sdk
│   ├── go
│   │   └── Testfile
│   ├── node
│   │   └── Testfile
│   └── ruby
│       └── Testfile
├── server
│   └── Testfile
```

We open a GitHub pull request to add a new feature to the SDKs:

```
sdk/go/account.go             | 10 +++++-----
sdk/go/account_test.go        | 10 +++++-----
sdk/node/src/account.ts       | 10 +++++-----
sdk/node/test/account.ts      | 10 +++++-----
sdk/ruby/lib/account.rb       | 10 +++++-----
sdk/ruby/spec/account_spec.rb | 10 +++++-----
6 files changed, 60 insertions(+), 60 deletions(-)
```

A `testbot farmer` process on a server
responds to the GitHub webhook by:

* identifying the directories containing files that have changed
* walking up the file hierarchy to find `Testfile`s for changed directories
* saving test jobs to its backing Postgres database

Each `Testfile` defines test jobs for its directory. Ours might be:

```
$ cat $ORG/sdk/go/Testfile
tests: cd $ORG/sdk && go test -cover ./...
$ cat $ORG/sdk/node/Testfile
tests: cd $ORG/sdk/node && npm install && npm test
$ cat $ORG/sdk/ruby/Testfile
tests: $ORG/sdk/ruby/test.sh
```

Each line contains a name and command
separated by a colon.
The name appears in GitHub checks.
The command is run by a `testbot worker`,
which is continuously long polling `testbot farmer` via HTTP over TLS,
asking for test jobs.

We run more `testbot worker` instances to increase test parallelism.

In this example,
the tests for the Go, Node, and Ruby SDKs
will begin to run almost simultaneously
as different `testbot worker` processes pick them up.

Tests for `dashboard` and `server` will not run in this pull request
because no files in their directories were changed.

## Credit

[Keith Rarick](https://xph.us/)
designed and implemented testbot in November 2017.
Since then, engineers at Chain, Interstellar, and Pogo
have been maintaining it and running it on a succession of internal monorepos.
