-- github data, reported from github, stored as-is

CREATE TABLE pr (
	num int PRIMARY KEY,
	head text NOT NULL
);

CREATE TABLE job (
	sha text NOT NULL,
	dir text NOT NULL,
	name text NOT NULL,

	-- We'd like to do this, but Postgres can't have
	-- a foreign key that references a non-unique column.
	-- Instead, we do a little extra work in resolve
	-- to delete jobs that don't correspond to any pr.
	-- See occurrences of job_garbage below.
	-- FOREIGN KEY (sha) REFERENCES pr (head) ON DELETE CASCADE,

	PRIMARY KEY (sha, dir, name)
);

-- worker box data, reported from workers, stored as-is

CREATE TABLE box (
	id text PRIMARY KEY,
	host text NOT NULL,
	last_seen_at timestamp NOT NULL DEFAULT now()
);

-- derived data

CREATE TABLE run (
	sha text NOT NULL,
	dir text NOT NULL,
	name text NOT NULL,
	UNIQUE (sha, dir, name),
	FOREIGN KEY (sha, dir, name) REFERENCES job ON DELETE CASCADE,

	box text NOT NULL,
	UNIQUE (box),
	FOREIGN KEY (box) REFERENCES box ON DELETE CASCADE
);

CREATE VIEW job_garbage AS
	SELECT sha, dir, name FROM job
	WHERE (sha) NOT IN (SELECT head FROM pr);

CREATE FUNCTION resolve() RETURNS trigger AS $$
DECLARE
	jcsha text;
	jcdir text;
	jcname text;
	bid text;
	n int;
BEGIN
	-- First, delete any jobs that don't correspond
	-- to a pr. See the foreign key comment in job.
	-- But don't do the delete at all if there's nothing
	-- to delete, because deleting zero rows still fires
	-- triggers and would be an unbounded recursion here.
	SELECT count(*) INTO strict n FROM job_garbage;
	IF n > 0 THEN
		DELETE FROM job
		WHERE (sha, dir, name) IN (TABLE job_garbage);
	END IF;

	-- Find one assignment and attempt to insert it.
	-- It's possible that a concurrent process inserts
	-- a different mapping for either the job or the box,
	-- causing this insertion to fail. That is okay.
	-- If we found one (regardless of whether we successfully
	-- insert it), our insert attempt will recursively trigger
	-- this resolve function to try again.
	-- This process will repeat until we reach a fixed point
	-- (that is, no new assignments are possible because there
	-- are no unassigned jobs or no boxes available),
	-- at which time we notify all observers.

	SELECT id INTO bid FROM box
	WHERE (id) NOT IN (SELECT box FROM run);

	SELECT sha, dir, name INTO jcsha, jcdir, jcname FROM job
	WHERE (sha, dir, name) NOT IN (SELECT sha, dir, name FROM run)
	ORDER BY length(dir) DESC;

	IF jcsha IS NULL OR bid IS NULL THEN
		NOTIFY state_wakeup;
	ELSE
		INSERT INTO run (sha, dir, name, box)
		VALUES (jcsha, jcdir, jcname, bid)
		ON CONFLICT DO NOTHING;
	END IF;

	RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER pr_write
	AFTER INSERT OR UPDATE OR DELETE ON pr
	EXECUTE PROCEDURE resolve();

CREATE TRIGGER job_write
	AFTER INSERT OR UPDATE OR DELETE ON job
	EXECUTE PROCEDURE resolve();

CREATE TRIGGER box_write
	AFTER INSERT OR UPDATE OR DELETE ON box
	EXECUTE PROCEDURE resolve();

CREATE TRIGGER run_write
	AFTER INSERT OR UPDATE OR DELETE ON run
	EXECUTE PROCEDURE resolve();

-- results, to report back to github

CREATE TABLE result (
	id serial PRIMARY KEY,
	sha text NOT NULL,
	dir text NOT NULL,
	name text NOT NULL,
	elapsed_ms int NOT NULL,
	pr int[] NOT NULL,
	state text NOT NULL, -- error, failure, pending, or success
	descr text NOT NULL, -- any extra info, shows up in GH web UI
	url text NOT NULL,
	reported boolean NOT NULL DEFAULT false,
	created_at timestamp NOT NULL DEFAULT now()
);

CREATE FUNCTION notify_report() RETURNS trigger AS $$
DECLARE
BEGIN
	NOTIFY report;
	RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER result_write
	AFTER INSERT OR UPDATE OR DELETE ON result
	EXECUTE PROCEDURE notify_report();

