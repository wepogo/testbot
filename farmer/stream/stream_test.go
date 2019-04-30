package stream

import (
	"context"
	"testing"
	"time"

	"github.com/wepogo/testbot"
)

func TestGetCancel(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()
	_, err := Get(ctx, "", testbot.Job{})
	if err != context.DeadlineExceeded {
		t.Fatal(err)
	}
}
