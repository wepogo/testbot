package farmer

const guide = `
Automated Testing Tool Guide

We practice "Continuous Integration" (CI), that is, we
automatically run a set of tests on every commit, before
we land it. We do this with an tool called testbot.

Testbot is oriented around pull requests. For any open
pull request, it runs tests on the commit at the head of
the pull request branch, and reports the results to
GitHub as "status" objects on that commit, which GitHub
then displays in its UI as a green dot or red X next to
the commit, as well as in more detail at the bottom of
the pull request page. When a new commit is pushed to
that branch, making the previous head commit obsolete,
testbot cancels any tests still running on obsolete
commits and starts running tests on the new head.


Quick Start

To add a new test, make a file called Testfile in the
directory where you want the test to run. The test will
run when any file changes anywhere in the tree rooted in
this directory. A Testfile looks like this:

    # this is a Testfile
    npmtest: npm test
    gotest: go test

Read on for more details.


Testfile Format

A Testfile contains one-line entries. Each entry defines
a test. An entry is an alphanumeric test name followed
by a colon followed by a shell command. For example:

    rubocop: bundle && bundle exec rubocop
    gctrace: GODEBUG=gctrace=1 go test

Lines beginning with # are ignored. Blank lines are
ignored. Lines that don't fit this format are an error.


Finding Tests

Here's how testbot finds tests to run.

It looks for a file called Testfile in every directory
affected by the pull request, and runs all the tests in
all the Testfiles it finds.

What does it mean for a directory to be "affected" by
the pull request? Specifically, it looks at the pull
request diff (from the nearest common ancestor of the
base branch and the pull request branch to the head of
the pull request branch) to find a list of all files
added, removed, renamed, or modified. It then considers
any ancestor directory of any of these files to be
affected. So, in a pull request that deleted a/b/c,
renamed d/e to f/g, created h/i, and edited but did not
rename or move the file j/k, the set of affected
directories would be /, /a, /a/b, /d, /f, /h, and /j.

Note in particular that merely deleting a file from a
directory will run the tests in that directory.

As a special case, an entry named "setup" will not run
as a test on its own. Instead, it will run before every
other test in its Testfile and the Testfile in any
parent directory. For parent-directory Testfiles, the
setup entry runs even if no files in the setup entry's
directory were affected. A consequence of all this is
there can be multiple setup tasks for a single test.
The setup tasks run in an arbitrary order; the only
guarantee is that they run sequentially and they all
finish before the test starts. If any setup task fails,
the test won't be run, and is considered to have failed.


Test Environment

Each test runs on a machine image derived from the stock
Ubuntu AMI, modified by $TESTED_REPO/testbot/Aptfile and
$TESTED_REPO/testbot/setup.sh.

The test runner runs each test in a controlled
environment:

- makes a fresh, clean checkout in a new workspace
- sets some environment variables
- runs the test command in the Testfile's directory

If the process exits with a 0 status, the test passes.

It collects output from the test process (by redirecting
both stdout and stderr to the same file on disk). When
the test finishes (either passing or failing), it saves
the output file in S3 and links to it from the "Details"
link on the pull request page.


Test Environment Nitty-Gritty

There is much to say here. Generally, no one needs to
worry about any of it.

Currently, the test process has minimal isolation. It's
run as an ordinary user (the same user as the test
runner!) in an ordinary directory (no chroot or pivot
root or mount namespace) with ordinary network access.
In the future, we might want to put the test in a more
aggressive sandbox, if only to make writing tests easier
(so test writers don't need to worry so much about
cleaning up after themselves).

The test runner starts each test process in a new Unix
process group. When the test process finishes, it sends
a KILL (9) signal to the process group to kill any child
processes started by the test. (But if the test starts
any of the child processes in a new process group, this
won't kill them, so they may linger even after the next
test has started. If this becomes a problem, we can fix
it by putting the test in a cgroup.)
`
