package presence

import (
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestFormat(t *testing.T) {
	opts := Options{LargeImage: "logo", LargeText: "UNBEATABLE"}
	arcade := State{V: 1, State: StateArcade, Song: "RE-TUNED", Artist: "peak divide ft. Rachel Lake",
		Difficulty: "UNBEATABLE", Level: ptr(20), HighScore: ptr(1155577), Rank: "B", StartedAt: 1727790000}

	cases := []struct {
		name          string
		in            State
		details       string
		state         string
		start         int64
		largeText     string
		wantNilResult bool
	}{
		{name: "arcade with PB", in: arcade,
			details: "RE-TUNED [UNBEATABLE Lv 20]", state: "PB: 1,155,577 (B)", start: 1727790000, largeText: "peak divide ft. Rachel Lake"},
		{name: "arcade without PB",
			in:      State{V: 1, State: StateArcade, Song: "Familiar Acoustic", Difficulty: "Hard", StartedAt: 5},
			details: "Familiar Acoustic [Hard]", state: "No PB yet", start: 5, largeText: "UNBEATABLE"},
		{name: "arcade paused drops timer",
			in:      func() State { s := arcade; s.Paused = true; return s }(),
			details: "RE-TUNED [UNBEATABLE Lv 20] (paused)", state: "PB: 1,155,577 (B)", largeText: "peak divide ft. Rachel Lake"},
		{name: "PB without grade",
			in:      State{V: 1, State: StateArcade, Song: "X", Difficulty: "Star", HighScore: ptr(999)},
			details: "X [Star]", state: "PB: 999", largeText: "UNBEATABLE"},
		{name: "results",
			in:      State{V: 1, State: StateResults, Song: "YEAH YEAH", Artist: "peak divide", Difficulty: "UNBEATABLE", Level: ptr(18), HighScore: ptr(1256914), Rank: "A"},
			details: "Results: YEAH YEAH [UNBEATABLE Lv 18]", state: "PB: 1,256,914 (A)", largeText: "peak divide"},
		{name: "song without difficulty",
			in:      State{V: 1, State: StateArcade, Song: "Custom"},
			details: "Custom", state: "No PB yet", largeText: "UNBEATABLE"},
		{name: "story with chapter",
			in:      State{V: 1, State: StateStory, Chapter: "Chapter 4", Stars: ptr(8.703)},
			details: "Story mode - Chapter 4", state: "★ 8.703", largeText: "UNBEATABLE"},
		{name: "story without chapter",
			in:      State{V: 1, State: StateStory},
			details: "Story mode", largeText: "UNBEATABLE"},
		{name: "story rhythm section keeps timer",
			in:      State{V: 1, State: StateStory, Chapter: "Chapter 2", Song: "Low", StartedAt: 42},
			details: "Story mode - Chapter 2", start: 42, largeText: "UNBEATABLE"},
		{name: "story rhythm section paused",
			in:      State{V: 1, State: StateStory, Chapter: "Chapter 2", Song: "Low", Paused: true},
			details: "Story mode - Chapter 2 (paused)", largeText: "UNBEATABLE"},
		{name: "idle with stars",
			in:      State{V: 1, State: StateIdle, Stars: ptr(12.345)},
			details: "In menus", state: "★ 12.345", largeText: "UNBEATABLE"},
		{name: "idle stars rounded to 3dp",
			in:      State{V: 1, State: StateIdle, Stars: ptr(7.0)},
			details: "In menus", state: "★ 7.000", largeText: "UNBEATABLE"},
		{name: "idle without stars",
			in:      State{V: 1, State: StateIdle},
			details: "In menus", largeText: "UNBEATABLE"},
		{name: "closed clears", in: State{V: 1, State: StateClosed}, wantNilResult: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Format(c.in, opts)
			if c.wantNilResult {
				if a != nil {
					t.Fatalf("got %+v, want nil", a)
				}
				return
			}
			if a == nil {
				t.Fatal("got nil activity")
			}
			if a.Details != c.details {
				t.Errorf("details = %q, want %q", a.Details, c.details)
			}
			if a.State != c.state {
				t.Errorf("state = %q, want %q", a.State, c.state)
			}
			var start int64
			if a.Timestamps != nil {
				start = a.Timestamps.Start
			}
			if start != c.start {
				t.Errorf("start = %d, want %d", start, c.start)
			}
			if a.Assets == nil || a.Assets.LargeImage != "logo" || a.Assets.LargeText != c.largeText {
				t.Errorf("assets = %+v, want logo / %q", a.Assets, c.largeText)
			}
		})
	}
}

func TestFormatNoImage(t *testing.T) {
	if a := Format(State{V: 1, State: StateIdle}, Options{}); a.Assets != nil {
		t.Errorf("assets = %+v, want nil without a LargeImage", a.Assets)
	}
}

func TestThousands(t *testing.T) {
	for in, want := range map[int]string{0: "0", 7: "7", 999: "999", 1000: "1,000", 123456: "123,456",
		1234567: "1,234,567", -1234: "-1,234"} {
		if got := thousands(in); got != want {
			t.Errorf("thousands(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFitField(t *testing.T) {
	long := strings.Repeat("★", 200)
	if got := []rune(fitField(long)); len(got) != maxField || got[len(got)-1] != '…' {
		t.Errorf("long field: %d runes, last %q", len(got), got[len(got)-1])
	}
	if got := fitField("A"); len([]rune(got)) < 2 {
		t.Errorf("short field %q not padded", got)
	}
	if fitField("") != "" {
		t.Error("empty field should stay empty")
	}
}

func TestParseState(t *testing.T) {
	s, err := ParseState([]byte(`{"v":1,"state":"arcade","song":"Low","highScore":123456,"accuracy":0.973,"rank":"S","stars":42.5,"paused":false,"startedAt":1727790000}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.Song != "Low" || *s.HighScore != 123456 || *s.Stars != 42.5 || s.StartedAt != 1727790000 {
		t.Errorf("parsed %+v", s)
	}
	for _, bad := range []string{`{"v":2,"state":"idle"}`, `{"v":1,"state":"dancing"}`, `not json`} {
		if _, err := ParseState([]byte(bad)); err == nil {
			t.Errorf("ParseState(%s) succeeded, want error", bad)
		}
	}
}
