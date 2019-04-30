// +build aws

package worker

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"

	"i10r.io/config/aws"
)

func init() {
	store, _, stack, err := aws.Store()
	if err != nil {
		log.Fatalf("querying parameter store: %s\n", err)
	}
	store.PathPrefix = fmt.Sprintf("/%s/testbot-worker/env/", stack)

	bucket = store.GetString("S3_BUCKET", "")
	netlify = store.GetString("NETLIFY_AUTH_TOKEN", "")

	creds := store.GetString("GIT_CREDENTIALS", "")

	usr, err := user.Current()
	if err != nil {
		log.Fatalf("getting current user: %s\n", err)
	}
	gitfile := usr.HomeDir + "/.git-credentials"

	// write credentials to ~/.git-credentials
	must(ioutil.WriteFile(gitfile, []byte(creds+"\n"), 0700))

	// update ~/.gitconfig to be configured to use ~/.git-credentials
	must(
		command(
			context.Background(),
			os.Stdout,
			"git",
			"config",
			"--global",
			"credential.helper",
			fmt.Sprintf("store --file %v", gitfile),
		).Run(),
	)
}
