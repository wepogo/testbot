// +build aws

package worker

import (
	"fmt"
	"log"

	"github.com/wepogo/testbot/config/aws"
)

func init() {
	store, _, stack, err := aws.Store()
	if err != nil {
		log.Fatalf("querying parameter store: %s\n", err)
	}
	store.PathPrefix = fmt.Sprintf("/%s/testbot-worker/env/", stack)

	region, err := aws.Region()
	if err == nil {
		regionS3 = region
	}

	bucket = store.GetString("S3_BUCKET", "")
	netlify = store.GetString("NETLIFY_AUTH_TOKEN", "")
	gitCredentials = store.GetString("GIT_CREDENTIALS", "")
}
