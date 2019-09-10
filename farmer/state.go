package farmer

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/lib/pq"

	"github.com/wepogo/testbot"
	"github.com/wepogo/testbot/log"
)

var (
	mu        sync.Mutex
	cond      = sync.Cond{L: &mu}
	allStates map[string]testbot.BoxState
	// NOTE(kr): allStates is reassigned, but never mutated
)

func notify(l *pq.Listener) {
	ctx := context.Background()

	err := loadAllBoxState(ctx)
	if err != nil {
		log.Fatalkv(ctx, "at", "boot loadAllBoxState", "error", err)
	}
	err = reportResults(ctx)
	if err != nil {
		log.Fatalkv(ctx, "at", "boot reportResults", "error", err)
	}

	for n := range l.Notify {
		switch n.Channel {
		case "state_wakeup":
			// Note that we can get spurious wakeups here,
			// for example when a new job is inserted but
			// no new assignments are possible.
			// In that case, the trigger will fire, be
			// unable to insert a row, and NOTIFIY on the
			// channel anyway.
			// That is not too bad though, because the total
			// state size is small. It'll just cause us to
			// reload the state only to find it's the same
			// as it was.
			err := loadAllBoxState(ctx)
			if err != nil {
				log.Error(ctx, err, "loadAllBoxState")
			}
		case "report":
			err := reportResults(ctx)
			if err != nil {
				log.Error(ctx, err, "reportResults")
			}
		}
	}
}

func gcBoxes() {
	const q = `
		DELETE FROM box
		WHERE last_seen_at < now() - '5s'::interval
	`
	for {
		time.Sleep(2 * time.Second)
		_, err := db.Exec(q)
		if err != nil {
			log.Error(context.Background(), err, "gc stale boxes")
		}
	}
}

func waitNewState(old testbot.BoxState) (new testbot.BoxState) {
	mu.Lock()
	defer mu.Unlock()
	new = getState(old.ID)
	for old == new {
		cond.Wait()
		new = getState(old.ID)
	}
	return new
}

func findBox(job testbot.Job) string {
	mu.Lock()
	defer mu.Unlock()
	for id, state := range allStates {
		if state.Job == job {
			return id
		}
	}
	return ""
}

// must hold mu on entry
func getState(id string) testbot.BoxState {
	s := allStates[id]
	s.ID = id // s could be the zero value
	return s
}

func loadAllBoxState(ctx context.Context) error {
	const q = `SELECT box, sha, dir, name FROM run`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("querying box state: %w", err)
	}

	newStates := make(map[string]testbot.BoxState)

	for rows.Next() {
		var box string
		var job testbot.Job
		err = rows.Scan(&box, &job.SHA, &job.Dir, &job.Name)
		if err != nil {
			return fmt.Errorf("scanning rows: %w", err)
		}
		newStates[box] = testbot.BoxState{ID: box, Job: job}
	}
	if rows.Err() != nil {
		return fmt.Errorf("rows.Err: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	defer cond.Broadcast()
	allStates = newStates
	return nil
}

func reportResults(ctx context.Context) error {
	q := `
		SELECT id, sha, dir, name, state, descr
		FROM result WHERE NOT reported
	`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return fmt.Errorf("querying unreported results: %w", err)
	}
	defer rows.Close()

	var reported []int64
	for rows.Next() {
		var id int64
		var state, desc string
		var job testbot.Job
		err = rows.Scan(&id, &job.SHA, &job.Dir, &job.Name, &state, &desc)
		if err != nil {
			return fmt.Errorf("scanning: %w", err)
		}
		err := postStatus(ctx, job, state, desc, selfURLf("result/%d", id))
		if err != nil {
			log.Error(ctx, err, "postStatus")
			continue // do not return here, keep going
		}
		reported = append(reported, id)
	}
	if rows.Err() != nil {
		return fmt.Errorf("rows.Err: %w", err)
	}

	q = `
		UPDATE result SET reported=true
		WHERE id = ANY($1::int[])
	`
	_, err = db.ExecContext(ctx, q, pq.Array(reported))
	if err != nil {
		return fmt.Errorf("updating result as reported: %w", err)
	}
	return nil
}

// upsertPR inserts or updates pr record for num
// to store head as the head commit.
// It returns whether the state was changed,
// that is, it returns true when a new record
// was inserted or if the existing record is
// updated, and false if the existing record
// already matches the value being stored
// (and was thus not modified).
func upsertPR(ctx context.Context, num int, head string) (bool, error) {
	const cq = `
		SELECT sha, dir, name FROM job, pr
		WHERE sha = head AND num = $1 AND head != $2
	`
	rows, err := db.QueryContext(ctx, cq, num, head)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var job testbot.Job
		err = rows.Scan(&job.SHA, &job.Dir, &job.Name)
		if err != nil {
			return false, err
		}
		go postPendingStatus(ctx, job, "canceled: obsolete commit")
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	const q = `
		INSERT INTO pr (num, head) VALUES ($1, $2)
		ON CONFLICT (num) DO UPDATE SET head=$2
		WHERE pr.head != $2
	`
	res, err := db.ExecContext(ctx, q, num, head)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func upsertJobs(ctx context.Context, sha, dir string, names []string) error {
	const q = `
		INSERT INTO job (sha, dir, name)
		VALUES ($1, $2, unnest($3::text[]))
		ON CONFLICT DO NOTHING
	`
	_, err := db.ExecContext(ctx, q, sha, dir, pq.Array(names))
	return err
}

type box struct {
	ID   string
	Host string
	Seen time.Time
}

func listBoxes(ctx context.Context) (b []box, err error) {
	const q = `SELECT id, host, last_seen_at FROM box ORDER BY id`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var box box
		err = rows.Scan(&box.ID, &box.Host, &box.Seen)
		if err != nil {
			return nil, err
		}
		b = append(b, box)
	}
	return b, rows.Err()
}

func listJobs(ctx context.Context) (j []testbot.Job, err error) {
	rows, err := db.QueryContext(ctx, `SELECT sha, dir, name FROM job`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var job testbot.Job
		err = rows.Scan(&job.SHA, &job.Dir, &job.Name)
		if err != nil {
			return nil, err
		}
		j = append(j, job)
	}
	return j, rows.Err()
}

type resultInfo struct {
	ID        int
	SHA       string
	Dir       string
	Name      string
	ElapsedMS int
	ElapsedSp string // for display
	Org       string
	Repo      string
	PR        []int64
	State     string
	Desc      string
	CreatedAt time.Time
}

func listResults(ctx context.Context, limit int) ([]resultInfo, error) {
	const q = `
		SELECT id, sha, dir, name, elapsed_ms, pr, state, descr, created_at
		FROM result
		ORDER BY id DESC
		LIMIT $1
	`
	return scanResults(db.QueryContext(ctx, q, limit))
}

func jobResults(ctx context.Context, job testbot.Job) ([]resultInfo, error) {
	const q = `
		SELECT id, sha, dir, name, elapsed_ms, pr, state, descr, created_at
		FROM result
		WHERE sha=$1 AND dir=$2 AND name=$3
		ORDER BY id DESC
	`
	return scanResults(db.QueryContext(ctx, q, job.SHA, job.Dir, job.Name))
}

func scanResults(rows *sql.Rows, err error) ([]resultInfo, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []resultInfo
	for rows.Next() {
		var result resultInfo
		err = rows.Scan(
			&result.ID,
			&result.SHA,
			&result.Dir,
			&result.Name,
			&result.ElapsedMS,
			pq.Array(&result.PR),
			&result.State,
			&result.Desc,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		result.ElapsedSp = pad(fmt.Sprintf("%v", time.Duration(result.ElapsedMS)*time.Millisecond))
		result.Org = org
		result.Repo = repo
		res = append(res, result)
	}
	return res, rows.Err()
}
