package presence

import (
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestFormatDefaultsToDetailed(t *testing.T) {
	s := State{V: 1, State: StateArcade, Song: "RE-TUNED", Artist: "peak divide", Difficulty: "Hard", Level: ptr(9)}
	got, want := Format(s, Options{}), Format(s, Options{Style: Detailed})
	if got.Details != want.Details || got.State != want.State {
		t.Errorf("nil style gave %q / %q, want %q / %q", got.Details, got.State, want.Details, want.State)
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
