package farmer

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"golang.org/x/xerrors"

	"github.com/wepogo/testbot"
)

const enspace = "\u2002"

// the GitHub "pull request object"
type prObj struct {
	Number int
	Head   struct{ SHA string }
}

type prEventReq struct {
	// Possible values:
	//   assigned unassigned review_requested
	//   review_request_removed labeled unlabeled opened
	//   edited closed reopened synchronize [sic]
	// We care about opened, closed, reopened, and synchronize.
	Action string
	PR     prObj `json:"pull_request"`
}

func createHook() error {
	// We use PubSubHubbub here because it is idempotent
	// (unlike the github webhook api).
	data := url.Values{
		"hub.mode":     {"subscribe"},
		"hub.topic":    {fmt.Sprintf("https://github.com/%s/%s/events/pull_request.json", org, repo)},
		"hub.callback": {selfURLf("pr-hook")},
		"hub.secret":   {hookSecret},
	}
	err := gh.Postf(data, nil, "/hub")
	if err != nil {
		err = xerrors.Errorf("unable to create hook. check $GITHUB_ORG [%s] or $GITHUB_REPO [%s]: %w", org, repo, err)
		return err
	}
	return nil
}

// note: postStatus should only be called
// from postPendingStatus and reportResults --
// all other functions should use
// postPendingStatus or markDone.
func postStatus(ctx context.Context, job testbot.Job, state, desc, url string) error {
	body := map[string]string{
		"state":       state, // error, failure, pending, or success
		"target_url":  url,
		"description": abbrevMiddle(desc, 140),
		"context":     job.Dir + enspace + job.Name,
	}
	err := gh.Postf(body, nil, "statuses/%s", job.SHA)
	if err != nil {
		// Sometimes this fails. Try once more.
		time.Sleep(250 * time.Millisecond)
		err = gh.Postf(body, nil, "statuses/%s", job.SHA)
	}
	return err
}

// abbrevMiddle returns a string with len <= n.
// If s is shorter than n, it return s.
// Otherwise, it removes len(s)-n+3 bytes from the
// middle of s and replaces them with "...".
func abbrevMiddle(s string, n int) string {
	if len(s) <= n {
		return s
	}
	dots := "..."
	if n < len(dots) {
		dots = dots[:n]
	}
	n -= len(dots)
	return s[:(n+1)/2] + dots + s[len(s)-n/2:]
}
