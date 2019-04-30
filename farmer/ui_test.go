package farmer

import (
	"testing"
	"time"
)

func TestReltime(t *testing.T) {
	const ε = time.Millisecond
	cases := []struct {
		d time.Duration
		s string
	}{
		{2 * time.Second, "<5s ago"},
		{5 * time.Second, "5s ago"},
		{119 * time.Second, "119s ago"},
		{120*time.Second - ε, "120s ago"},
		{120 * time.Second, "2m ago"},
		{150*time.Second - ε, "2m ago"},
		{150 * time.Second, "3m ago"},
		{180 * time.Second, "3m ago"},
		{5 * time.Minute, "5m ago"},
		{119 * time.Minute, "119m ago"},
		{120*time.Minute - ε, "120m ago"},
		{120 * time.Minute, "2h ago"},
		{150*time.Minute - ε, "2h ago"},
		{150 * time.Minute, "3h ago"},
		{180 * time.Minute, "3h ago"},
		{5 * time.Hour, "5h ago"},
		{24 * time.Hour, "24h ago"},
		{48*time.Hour - ε, "48h ago"},
		{48 * time.Hour, "2d ago"},
		{5 * 24 * time.Hour, "5d ago"},
		{14 * 24 * time.Hour, "14d ago"},
		{90*24*time.Hour - ε, "90d ago"},
	}

	for _, test := range cases {
		got := reltime(time.Now().Add(-test.d))
		if got != test.s {
			t.Errorf("reltime(now-%v) = %q, want %q", test.d, got, test.s)
		}
	}

	d := 90 * 24 * time.Hour
	at := time.Now().Add(-d)
	got := reltime(at)
	want := at.Format("Jan 2006")
	if got != want {
		t.Errorf("reltime(now-%v) = %q, want %q", d, got, want)
	}
}

func TestPad(t *testing.T) {
	cases := []struct {
		s string
		w int
	}{
		{"680", 2},
		{"3127", 1},
		{"33905", 0},
		{"241471", 0},
	}

	for _, test := range cases {
		g := len(pad(test.s))
		if g != test.w {
			t.Errorf("len(pad(%q, 5)) = %d, want %d", test.s, g, test.w)
		}
	}
}
