package presence

import "testing"

func TestDetailedStyle(t *testing.T) {
	opts := Options{LargeImage: "logo", LargeText: "UNBEATABLE", Style: Detailed}
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
			details: "RE-TUNED - peak divide ft. Rachel Lake", state: "UNBEATABLE Lv 20 · PB: 1,155,577 (B)", start: 1727790000, largeText: "UNBEATABLE"},
		{name: "arcade without PB",
			in:      State{V: 1, State: StateArcade, Song: "Familiar Acoustic", Difficulty: "Hard", StartedAt: 5},
			details: "Familiar Acoustic", state: "Hard · No PB yet", start: 5, largeText: "UNBEATABLE"},
		{name: "arcade paused drops timer",
			in:      func() State { s := arcade; s.Paused = true; return s }(),
			details: "RE-TUNED - peak divide ft. Rachel Lake (paused)", state: "UNBEATABLE Lv 20 · PB: 1,155,577 (B)", largeText: "UNBEATABLE"},
		{name: "PB without grade",
			in:      State{V: 1, State: StateArcade, Song: "X", Difficulty: "Star", HighScore: ptr(999)},
			details: "X ", state: "Star · PB: 999", largeText: "UNBEATABLE"},
		{name: "results",
			in:      State{V: 1, State: StateResults, Song: "YEAH YEAH", Artist: "peak divide", Difficulty: "UNBEATABLE", Level: ptr(18), HighScore: ptr(1256914), Rank: "A"},
			details: "Results: YEAH YEAH - peak divide", state: "UNBEATABLE Lv 18 · PB: 1,256,914 (A)", largeText: "UNBEATABLE"},
		{name: "song without artist, difficulty or level",
			in:      State{V: 1, State: StateArcade, Song: "Custom"},
			details: "Custom", state: "No PB yet", largeText: "UNBEATABLE"},
		{name: "level without difficulty",
			in:      State{V: 1, State: StateArcade, Song: "Custom", Artist: "me", Level: ptr(3)},
			details: "Custom - me", state: "Lv 3 · No PB yet", largeText: "UNBEATABLE"},
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

func TestSpotifyStyle(t *testing.T) {
	opts := Options{Style: Spotify}
	cases := []struct {
		name    string
		in      State
		details string
		state   string
		start   int64
	}{
		{name: "arcade",
			in:      State{V: 1, State: StateArcade, Song: "RE-TUNED", Artist: "peak divide ft. Rachel Lake", Difficulty: "UNBEATABLE", Level: ptr(20), HighScore: ptr(1155577), StartedAt: 7},
			details: "RE-TUNED", state: "peak divide ft. Rachel Lake", start: 7},
		{name: "arcade paused",
			in:      State{V: 1, State: StateArcade, Song: "RE-TUNED", Artist: "peak divide", Paused: true, StartedAt: 7},
			details: "RE-TUNED (paused)", state: "peak divide"},
		{name: "song without artist",
			in:      State{V: 1, State: StateArcade, Song: "Custom"},
			details: "Custom"},
		{name: "results",
			in:      State{V: 1, State: StateResults, Song: "YEAH YEAH", Artist: "peak divide", HighScore: ptr(1)},
			details: "YEAH YEAH", state: "peak divide"},
		{name: "story rhythm section",
			in:      State{V: 1, State: StateStory, Chapter: "Chapter 2", Song: "Low", Artist: "peak divide", StartedAt: 42},
			details: "Low", state: "peak divide", start: 42},
		{name: "story walking around",
			in:      State{V: 1, State: StateStory, Chapter: "Chapter 4", Stars: ptr(8.7)},
			details: "Story mode", state: "Chapter 4"},
		{name: "menus",
			in:      State{V: 1, State: StateIdle, Stars: ptr(8.7)},
			details: "In menus", state: "★ 8.700"},
		{name: "menus without stars",
			in:      State{V: 1, State: StateIdle},
			details: "In menus"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Format(c.in, opts)
			if a.Details != c.details || a.State != c.state {
				t.Errorf("got %q / %q, want %q / %q", a.Details, a.State, c.details, c.state)
			}
			var start int64
			if a.Timestamps != nil {
				start = a.Timestamps.Start
			}
			if start != c.start {
				t.Errorf("start = %d, want %d", start, c.start)
			}
		})
	}
	if a := Format(State{V: 1, State: StateClosed}, opts); a != nil {
		t.Errorf("closed: got %+v, want nil", a)
	}
}

func TestLookupStyle(t *testing.T) {
	if _, ok := Styles[DefaultStyle]; !ok {
		t.Fatalf("default style %q is not registered", DefaultStyle)
	}
	for _, name := range StyleNames() {
		if _, err := LookupStyle(name); err != nil {
			t.Errorf("LookupStyle(%q): %v", name, err)
		}
	}
	if _, err := LookupStyle("Spotify"); err != nil {
		t.Errorf("lookup should ignore case: %v", err)
	}
	if _, err := LookupStyle("nope"); err == nil {
		t.Error("LookupStyle(nope) succeeded, want error")
	}
}
