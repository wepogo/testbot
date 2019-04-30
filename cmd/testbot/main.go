package main

import (
	"fmt"
	"os"

	"github.com/wepogo/testbot"
	"github.com/wepogo/testbot/farmer"
	"github.com/wepogo/testbot/worker"
)

func main() {
	if n := len(os.Args); n < 2 || n != needArgs[os.Args[1]] {
		usage()
	}
	switch os.Args[1] {
	case "farmer":
		farmer.Main()
	case "worker":
		worker.Main()
	case "onejob":
		worker.OneJob(testbot.Job{
			SHA:  os.Args[2],
			Dir:  os.Args[3],
			Name: os.Args[4],
		})
	default:
		usage()
	}
}

func usage() {
	fmt.Fprint(os.Stderr, usageString)
	os.Exit(2)
}

const usageString = `usage:
  testbot farmer
  testbot worker
  testbot onejob [sha] [dir] [name]

For onejob, sha is a git commit hash, dir is the location
of a Testfile relative to $I10R, and name is the name
of an entry in the Testfile.

Example:
  $ testbot onejob e3e9378da testbot gotest
`

var needArgs = map[string]int{"farmer": 2, "worker": 2, "onejob": 5}
