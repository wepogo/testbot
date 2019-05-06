// Package testbot contains type declarations used across
// multiple testbot-related packages.
package testbot

import (
	"errors"
	"path"
	"strings"
	"time"
)

type BoxPingReq struct {
	ID   string
	Host string
}

type Job struct {
	SHA  string
	Dir  string
	Name string
}

func ParseJob(s string) (j Job, err error) {
	i := strings.IndexByte(s, '/')
	if i < 0 {
		return Job{}, errors.New("bad job")
	}
	j.SHA = s[:i]
	if strings.TrimLeft(j.SHA, "0123456789abcdefABCDEF") != "" {
		return Job{}, errors.New("bad job: non-hex char in commit hash")
	}
	j.Dir, j.Name = path.Split(s[i:])
	j.Dir = strings.TrimRight(j.Dir, "/")
	if j.Dir == "" {
		j.Dir = "/"
	}
	if j.Name == "" {
		return Job{}, errors.New("bad job: no name")
	}
	return j, nil
}

type BoxState struct {
	ID  string
	Job Job
}

type BoxJobUpdateReq struct {
	Job     Job
	Status  string
	Desc    string
	URL     string
	Elapsed time.Duration
}

type RetryReq struct {
	ResultID int
}

type CancelReq struct {
	Job Job
}
