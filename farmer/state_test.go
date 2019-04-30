package farmer

import (
	"context"
	"reflect"
	"testing"

	"i10r.io/database/pg/pgtest"
	"i10r.io/testbot"
)

func TestSchema(t *testing.T) {
	ctx := context.Background()
	_, db = pgtest.NewDB(t, "schema.sql")
	must(t, boxPing(ctx, testbot.BoxPingReq{ID: "box1"}))
	_, err := upsertPR(ctx, 1, "commit1")
	must(t, err)
	must(t, upsertJobs(ctx, "commit1", "/", []string{"cmd1"}))
	checkRuns(t, run{"commit1", "/", "cmd1", "box1"})

	job := testbot.Job{SHA: "commit1", Dir: "/", Name: "cmd1"}
	must(t, markDone(ctx, job, "error", "canceled by operator", "", "", 0))
	checkRuns(t) // should be none
}

type run struct {
	sha, dir, name string
	box            string
}

func checkRuns(t *testing.T, want ...run) {
	var got []run
	rows, err := db.Query("SELECT sha, dir, name, box FROM run")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var run run
		err = rows.Scan(&run.sha, &run.dir, &run.name, &run.box)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, run)
	}
	if rows.Err() != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("runs = %+v, want %+v", got, want)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
