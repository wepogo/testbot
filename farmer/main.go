package farmer

/*

Theory of Operation

The farmer process coordinates activity between GitHub
and test-runner processes.

On the GitHub side, it receives events for Pull Request
state changes (opened, closed, reopened, synchronize [sic])
and sends updates of the status of zero or more tests on
each commit.

On the test-runner side, it listens for boxes to
announce they are (still) alive (with a ping message),
to request work to do (with a longpoll message, doling
out jobs as they become available), and to report the
status of finished jobs (with a runstatus message).

Most of the data model is represented in the Postgres
schema. Some tables (pr and job) mirror the state of
GitHub, recording the events GitHub sends and a list of
jobs for each commit the farmer retrieves from files in
the repo. Other tables (box) mirror the state of
test-runner boxes, recording the messages they send. (As
of this writing, that is just the fact that a box exists
and when it was last heard from.)

Postgres triggers assign jobs to boxes whenever new jobs
appear or boxes become available (either because an
existing job finishes or because a new box comes
online). Foreign keys remove assignments whenever jobs
are deleted (when a job is finished or canceled) and
whenever boxes are deleted (because they've stopped
pinging). At any given time, the run table shows the
current assignment of jobs to boxes. Whenever the
assignment for a particular box changes (from no job to
a job, from a job to no job, or from one job to
another), farmer notifies the box of its new box state
via the pending or next longpoll request.

When a job runs to completion (success or failure), the
runstatus handler atomically moves the job to the result
table (deleting it from job and inserting it in result)
along with its final state.

Since Postgres is responsible for maintaining
consistency of the data model, the Go code in farmer is
mainly concerned with plumbing: listening for HTTP
requests, inserting and deleting records when
appropriate, and synchronizing responses to state
changes (by LISTENing to NOTIFY messages sent by
triggers in stored procedures and diffing state in
memory).

*/

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	_ "github.com/heroku/x/hmetrics/onload"
	"github.com/kr/githubauth"
	"github.com/lib/pq"
	"golang.org/x/xerrors"

	"github.com/wepogo/testbot"
	"github.com/wepogo/testbot/farmer/stream"
	"github.com/wepogo/testbot/github"
	"github.com/wepogo/testbot/httpjson"
	"github.com/wepogo/testbot/log"
)

const (
	testfile        = "Testfile"
	minReconnectDur = 20 * time.Millisecond
	maxReconnectDur = 20 * time.Second
	waitTimeout     = 25 * time.Second // less than 30s Heroku timeout
)

func or(v, d string) string {
	if v == "" {
		v = d
	}
	return v
}

var (
	// TODO(tmaher): move secrets into EC2 parameter store.
	baseURLStr  = os.Getenv("FARMER_URL")
	dumpReqsStr = os.Getenv("DUMP")
	dbURL       = os.Getenv("DATABASE_URL")
	hookSecret  = os.Getenv("HOOK_SECRET")
	org         = os.Getenv("GITHUB_ORG")
	repo        = os.Getenv("GITHUB_REPO")
	ghToken     = github.Token(os.Getenv("GITHUB_TOKEN"))
	listenAddr  = or(os.Getenv("LISTEN"), ":1994")
)

var baseURL *url.URL
var dumpReqs bool
var db *sql.DB
var gh = github.Open(
	ghToken,
	github.Repo(org, repo),
	// "raw" needed for fetching repo file contents
	github.Accept("application/vnd.github.raw+json"),
)
var httpClient = new(http.Client)

func Main() {
	var err error
	baseURL, err = url.Parse(baseURLStr)
	if err != nil {
		log.Fatalkv(context.Background(), "variable", "FARMER_URL", log.Error, err)
		os.Exit(1)
	}
	dumpReqs, err = strconv.ParseBool(dumpReqsStr)
	if len(dumpReqsStr) > 0 && err != nil {
		log.Fatalkv(context.Background(), "variable", "DUMP", log.Error, err)
		os.Exit(1)
	}

	err = createHook()
	if err != nil {
		log.Fatalkv(context.Background(), "error", err)
	}

	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalkv(context.Background(), "error", err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatalkv(
			context.Background(),
			"error",
			"database not connected. check DATABASE_URL. "+err.Error(),
		)
	}

	listener := pq.NewListener(dbURL, minReconnectDur, maxReconnectDur, nil)
	err = listener.Listen("state_wakeup")
	if err != nil {
		log.Fatalkv(context.Background(), "error", err)
	}
	err = listener.Listen("report")
	if err != nil {
		log.Fatalkv(context.Background(), "error", err)
	}
	go notify(listener)
	go gcBoxes()
	go initSync(listenAddr) // get initial PR state

	// browser-accessible URLs need github auth
	authMux := new(http.ServeMux)
	authMux.Handle("/guide.txt", static("guide.txt", guide))
	authMux.HandleFunc("/cancel", cancel)
	authMux.HandleFunc("/result/", result)
	authMux.HandleFunc("/live/", live)
	authMux.HandleFunc("/retry", retry)
	authMux.HandleFunc("/", index)

	mux := new(http.ServeMux)
	mux.Handle("/pr-hook", github.Hook(hookSecret, jsonHandler(prHook)))
	mux.Handle("/box-ping", jsonHandler(boxPing))           // TODO(tmaher): add HTTP basic auth
	mux.Handle("/box-longpoll", jsonHandler(boxLongPoll))   // TODO(tmaher): add HTTP basic auth
	mux.Handle("/box-runstatus", jsonHandler(boxRunStatus)) // TODO(tmaher): add HTTP basic auth
	mux.Handle("/box-livepoll", jsonHandler(boxLivePoll))
	mux.HandleFunc("/box-livesend", boxLiveSend)
	mux.Handle("/static/a.css", static("a.css", css))
	mux.Handle("/static/a.js", static("a.js", js))
	mux.Handle("/", githubauthHandler(authMux))

	var h http.Handler = mux
	if dumpReqs {
		h = dumpHandler{h}
	}
	err = http.ListenAndServe(listenAddr, h)
	log.Fatalkv(context.Background(), "error", err)
}

func initSync(listenAddr string) {
	ctx := context.Background()

	// Wait for our own server to be online, so there's no window
	// during which events could be missed between the initial sync
	// and when we start receiving events.
	addr := listenAddr
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	for !ping(addr) {
		time.Sleep(10 * time.Millisecond)
	}

	var prs []prObj
	err := gh.GetAllf(&prs, "pulls?state=open")
	if err != nil {
		log.Fatalkv(ctx, "at", "initial sync", "error", err)
	}
	for _, pr := range prs {
		err = populateJobs(ctx, pr)
		if err != nil {
			log.Fatalkv(ctx, "at", "initial sync", "error", err)
		}
	}
}

// ping dials addr using tcp, and returns
// whether the connection was established.
func ping(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func index(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}

	var v struct {
		Boxes  []box
		ErrBox error

		Jobs   []testbot.Job
		ErrJob error

		Results   []resultInfo
		ErrResult error

		States map[string]testbot.BoxState
	}

	mu.Lock()
	v.States = allStates
	mu.Unlock()

	v.Boxes, v.ErrBox = listBoxes(req.Context())
	v.Jobs, v.ErrJob = listJobs(req.Context())
	v.Results, v.ErrResult = listResults(req.Context(), 200)

	w.Header().Set("Content-Language", "en")
	err := homePage.Execute(w, v)
	if err != nil {
		log.Error(req.Context(), err, "result template")
	}
}

func result(w http.ResponseWriter, req *http.Request) {
	p := strings.TrimPrefix(req.URL.Path, "/result/")
	n, err := strconv.Atoi(p)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	const q = `SELECT url, sha, dir, name, pr FROM result WHERE id = $1`
	var u, sha, dir, name string
	var pr []int64
	err = db.QueryRow(q, n).Scan(&u, &sha, &dir, &name, pq.Array(&pr))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// TODO(kr): detect if the job can't be rerun
	// (for example, if the PR has been closed) and
	// hide the retry button.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Language", "en")
	data := struct {
		Title string
		PR    []int64
		Org   string
		Repo  string
	}{fmt.Sprintf("%.8s %s %s", sha, dir, name), pr, org, repo}
	err = resultPage.Execute(w, data)
	if err != nil {
		log.Error(req.Context(), err, "result template") // but continue
	}
	if f, ok := w.(http.Flusher); ok {
		// Show at least the retry button immediately,
		// even with poor connectivity to output storage.
		f.Flush()
	}

	if u == "" {
		io.WriteString(w, "sorry, no output is available for this test")
		return
	}
	u = workaroundDNS(u) // TODO(kr): remove workaround
	resp, err := httpClient.Get(u)
	if err != nil {
		io.WriteString(w, "fetching output: "+err.Error())
		return
	}
	defer resp.Body.Close()
	io.Copy(escapeWriter{w}, resp.Body)
}

func live(w http.ResponseWriter, req *http.Request) {
	p := strings.TrimPrefix(req.URL.Path, "/live/")
	job, err := testbot.ParseJob(p)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	const q = `
		SELECT count(*) > 0 FROM job
		WHERE sha = $1 AND dir = $2 AND name = $3
	`
	var isLive bool
	err = db.QueryRow(q, job.SHA, job.Dir, job.Name).Scan(&isLive)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var pr []int64
	err = db.QueryRow(`SELECT array_agg(num) FROM pr WHERE head = $1`, job.SHA).Scan(pq.Array(&pr))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Language", "en")
	data := struct {
		Title string
		PR    []int64
		Org   string
		Repo  string
		Live  bool

		Results   []resultInfo
		ErrResult error
	}{
		Title: fmt.Sprintf("%.8s %s %s", job.SHA, job.Dir, job.Name),
		PR:    pr,
		Org:   org,
		Repo:  repo,
		Live:  isLive,
	}

	data.Results, data.ErrResult = jobResults(req.Context(), job)
	err = livePage.Execute(w, data)
	if err != nil {
		log.Error(req.Context(), err, "live template")
		// continue, don't return
	}

	if !isLive {
		return
	}
	boxID := findBox(job)
	if boxID == "" {
		fmt.Fprintln(w, "in queue")
		return
	}
	flush := func() {}
	if f, ok := w.(http.Flusher); ok {
		// Show at least the cancel button immediately,
		// even with poor connectivity to the worker box.
		f.Flush()

		// Flush all job output immediately as we write it.
		flush = f.Flush
	}
	ctx := req.Context()
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	rc, err := stream.Get(ctx, boxID, job)
	if err != nil {
		fmt.Fprintf(escapeWriter{w}, "fetching output: %v\n", err)
		return
	}
	defer rc.Close()
	io.Copy(testbot.FlushWriter(escapeWriter{w}, flush), rc)
	io.WriteString(w, "\n<b>eof</b>\n")
}

func boxLiveSend(w http.ResponseWriter, req *http.Request) {
	boxID := req.Header.Get("Box-ID")
	job := testbot.Job{
		SHA:  req.Header.Get("Job-SHA"),
		Dir:  req.Header.Get("Job-Dir"),
		Name: req.Header.Get("Job-Name"),
	}
	stream.Send(boxID, job, req.Body)
}

func boxLivePoll(ctx context.Context, box struct{ ID string }) testbot.Job {
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	return stream.Poll(ctx, box.ID)
}

func prHook(ctx context.Context, ev prEventReq) error {
	if dumpReqs {
		log.Printkv(ctx, "ev", ev)
	}
	switch ev.Action {
	case "opened", "reopened", "synchronize":
		return populateJobs(ctx, ev.PR)
	case "closed":
		const q = `DELETE FROM pr WHERE num=$1`
		_, err := db.ExecContext(ctx, q, ev.PR.Number)
		return err
	}
	return nil
}

func boxPing(ctx context.Context, p testbot.BoxPingReq) error {
	q := `
		INSERT INTO box (id, host) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET last_seen_at = now(), host = $2
	`
	_, err := db.ExecContext(ctx, q, p.ID, p.Host)
	if err != nil {
		return xerrors.Errorf("insert box: %s", err)
	}
	return nil
}

func boxLongPoll(ctx context.Context, old testbot.BoxState) testbot.BoxState {
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	c := make(chan testbot.BoxState, 1)
	go func() { c <- waitNewState(old) }()
	select {
	case new := <-c:
		return new
	case <-ctx.Done():
		httpjson.ResponseWriter(ctx).WriteHeader(202)
		return old
	}
}

func boxRunStatus(ctx context.Context, req testbot.BoxJobUpdateReq) error {
	switch req.Status {
	case "pending":
		return postPendingStatus(ctx, req.Job, req.Desc)
	default:
		return markDone(ctx, req.Job, req.Status, req.Desc, req.URL, req.TraceURL, req.Elapsed)
	}
}

func cancel(w http.ResponseWriter, req *http.Request) {
	var rr testbot.CancelReq
	prefix := selfURLf("live") + "/"

	if ref := req.Header.Get("Referer"); strings.HasPrefix(ref, prefix) {
		var err error
		rr.Job, err = testbot.ParseJob(ref[len(prefix):])
		if err != nil {
			http.Error(w, "bad referer: "+err.Error(), 400)
			return
		}
	} else {
		// TODO(kr): maybe remove this case some day if no-body is using it
		err := json.NewDecoder(req.Body).Decode(&rr)
		if err != nil {
			http.Error(w, "bad request body: "+err.Error(), 400)
			return
		}
	}

	err := markDone(req.Context(), rr.Job, "error", "canceled by operator", "", "", 0)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

// state must be one of: error, failure, pending, success
func markDone(ctx context.Context, job testbot.Job, state, desc, url, traceURL string, elapsed time.Duration) error {
	const q = `
		WITH done AS (
			DELETE FROM job
			WHERE sha=$1 AND dir=$2 AND name=$3
			RETURNING sha, dir, name
		),
		donepr AS (
			SELECT sha, dir, name, array_agg(num) as prnum
			FROM done JOIN pr ON (done.sha=pr.head)
			GROUP BY sha, dir, name
		)
		INSERT INTO result (sha, dir, name, pr, state, descr, url, elapsed_ms, trace_url)
		SELECT sha, dir, name, prnum, $4, $5, $6, $7, $8 FROM donepr
	`
	ms := int(elapsed / time.Millisecond)
	_, err := db.ExecContext(ctx, q, job.SHA, job.Dir, job.Name, state, desc, url, ms, traceURL)
	return err
}

func postPendingStatus(ctx context.Context, job testbot.Job, desc string) error {
	url := selfURLf("live/%s/%s/%s", job.SHA, job.Dir, job.Name)
	return postStatus(ctx, job, "pending", desc, url)
}

func retry(w http.ResponseWriter, req *http.Request) {
	var rr testbot.RetryReq
	prefix := selfURLf("result") + "/"
	if ref := req.Header.Get("Referer"); strings.HasPrefix(ref, prefix) {
		var err error
		rr.ResultID, err = strconv.Atoi(ref[len(prefix):])
		if err != nil {
			http.Error(w, "bad referer: "+err.Error(), 400)
			return
		}
	} else {
		// TODO(kr): maybe remove this case some day if no-body is using it
		err := json.NewDecoder(req.Body).Decode(&rr)
		if err != nil {
			http.Error(w, "bad request body: "+err.Error(), 400)
			return
		}
	}
	const q = `
		INSERT INTO job (sha, dir, name)
		SELECT sha, dir, name FROM result
		WHERE id=$1
		ON CONFLICT (sha, dir, name) DO UPDATE SET sha=job.sha
		RETURNING sha, dir, name
	`
	var sha, dir, name string
	err := db.QueryRowContext(req.Context(), q, rr.ResultID).Scan(&sha, &dir, &name)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// TODO(kr): detect if the job won't actually run
	// (for example, if the PR has been closed) and
	// give a suitable message.
	url := selfURLf("live/%s/%s/%s", sha, dir, name)
	http.Redirect(w, req, url, http.StatusSeeOther)
}

func githubauthHandler(h http.Handler) http.Handler {
	var keys []*[32]byte
	for _, s := range strings.Split(os.Getenv("SECURE_KEY"), ",") {
		k := sha256.Sum256([]byte(s))
		keys = append(keys, &k)
	}

	return &githubauth.Handler{
		RequireOrg:   org,
		MaxAge:       28 * 24 * time.Hour, // match securekey Heroku addon key lifetime
		Keys:         keys,
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		Handler:      h,
	}
}

func jsonHandler(f interface{}) http.Handler {
	h, err := httpjson.Handler(f, errFunc)
	if err != nil {
		panic(err)
	}
	return h
}

func errFunc(ctx context.Context, w http.ResponseWriter, err error) {
	log.Error(ctx, err, "responding http status 500")
	http.Error(w, err.Error(), 500)
}

func selfURLf(format string, arg ...interface{}) string {
	u := *baseURL
	u.Path = path.Clean("/" + fmt.Sprintf(format, arg...))
	return u.String()
}

type dumpHandler struct{ h http.Handler }

func (h dumpHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// dump body for everything but gh events; they are too noisy
	dump, err := httputil.DumpRequest(req, req.URL.Path != "/pr-hook")
	if err != nil {
		log.Error(req.Context(), err)
	} else {
		os.Stderr.Write(dump)
	}
	h.h.ServeHTTP(w, req)
}

// temporary workaround until we get
// our DNS and TLS set up for S3.
func workaroundDNS(u string) string {
	const s = "https://farmer-ci-logs.chainaws.net/"
	if strings.HasPrefix(u, s) {
		// note the scheme change to HTTP (not HTTPS)
		u = "http://farmer-ci-logs.chainaws.net.s3.amazonaws.com/" + u[len(s):]
	}
	return u
}

type escapeWriter struct {
	w io.Writer
}

func (w escapeWriter) Write(p []byte) (int, error) {
	template.HTMLEscape(w.w, p)
	return len(p), nil // HTMLEscape has no return value ¯\_(ツ)_/¯
}
