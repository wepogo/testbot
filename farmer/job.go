package farmer

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"time"

	"golang.org/x/xerrors"

	"github.com/wepogo/testbot"
	"github.com/wepogo/testbot/github"
	"github.com/wepogo/testbot/log"
)

// populateJobs gets the list of files
// so we know which dirs were affected, then
// uses the list of dirs to pull the Testfiles and find
// test jobs to run.
// The initial file list is retrieved synchronously,
// but the rest is done in a background goroutine.
func populateJobs(ctx context.Context, pr prObj) error {
	modified, err := upsertPR(ctx, pr.Number, pr.Head.SHA)
	if err != nil {
		return xerrors.Errorf("upserting pr: %w", err)
	}
	if !modified {
		return nil // nothing new to do
	}

	// NOTE(kr): this is a race: if the pull request is
	// modified after we get the event but before we send this
	// RPC, we'll get wrong results here. However, this is okay,
	// because we'll get another event for the new HEAD and
	// correctly populate that one, and the jobs for this SHA
	// will need to be canceled anyway.
	var files []struct{ Filename string }
	err = gh.GetAllf(&files, "pulls/%d/files", pr.Number)
	for err == github.StatusError(404) {
		// The GitHub API may 404 for a PR they just delivered
		// a webhook for. Retry until it succeeds. Under normal
		// operations, a PR can't be deleted (only closed), so
		// this is safe to retry.
		err = gh.GetAllf(&files, "pulls/%d/files", pr.Number)
	}
	if err != nil {
		return xerrors.Errorf("getting pr files: %w", err)
	}
	var dirs []string
	for _, file := range files {
		dirs = append(dirs, path.Dir("/"+file.Filename))
	}
	var testfiles []string
	for _, dir := range fillParents(dirs) {
		testfiles = append(testfiles, path.Join(dir, testfile))
	}

	go populateJobsBG(pr.Head.SHA, testfiles)
	return nil
}

func populateJobsBG(sha string, files []string) {
	ctx := context.Background()
	var failed []string
	for _, file := range files {
		dir := path.Dir(file)
		var body bytes.Buffer
		log.Printkv(ctx, "at", "fetch", "path", file, "ref", sha)
		err := gh.Getf(&body, "contents/%s?ref=%s", file, sha)
		if err == github.StatusError(404) {
			continue
		} else if err != nil {
			failed = append(failed, file)
			log.Error(ctx, err)
			continue
		}

		entries, err := testbot.ParseTestfile(&body)
		if err, ok := err.(testbot.SyntaxError); ok {
			job := testbot.Job{SHA: sha, Dir: dir, Name: testfile}
			fileURL := fmt.Sprintf("https://github.com/%s/%s/blob/%s%s", org, repo, sha, file)
			postStatus(ctx, job, "error", err.Error(), fileURL)
			continue
		}
		if err != nil {
			failed = append(failed, file)
			log.Error(ctx, err)
			continue
		}
		var names []string
		for name := range entries {
			names = append(names, name)
		}
		err = upsertJobs(ctx, sha, dir, names)
		if err != nil {
			failed = append(failed, file)
			log.Error(ctx, err)
			continue
		}
		for _, name := range names {
			job := testbot.Job{SHA: sha, Dir: dir, Name: name}
			postPendingStatus(ctx, job, "in queue")
		}
		_ = file
	}
	if len(failed) > 0 {
		time.Sleep(time.Second)
		go populateJobsBG(sha, failed)
	}
}

func fillParents(dirs []string) []string {
	var allDirs []string
	for _, dir := range dirs {
		for dir != "/" && dir != "." {
			allDirs = append(allDirs, dir)
			dir = path.Dir(dir)
		}
		allDirs = append(allDirs, dir)
	}
	sort.Strings(allDirs)
	return uniq(allDirs)
}

// uniq returns a copy of s with adjacent duplicate elements removed.
// If you need all duplicates removed, consider sorting s first.
func uniq(s []string) []string {
	var a []string
	for _, v := range s {
		if len(a) == 0 || a[len(a)-1] != v {
			a = append(a, v)
		}
	}
	return a
}
