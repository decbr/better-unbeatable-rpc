package presence

import (
	"strings"
	"testing"
)

func lines(t *testing.T, c CustomStyle, s State) (string, string) {
	t.Helper()
	style, err := c.Compile()
	if err != nil {
		t.Fatal(err)
	}
	return style(s)
}

func TestTemplateRender(t *testing.T) {
	full := State{V: 1, State: StateArcade, Song: "RE-TUNED", Artist: "peak divide", Charter: "re-kouned",
		Difficulty: "UNBEATABLE", Level: ptr(20), HighScore: ptr(1155577), Rank: "B", Stars: ptr(8.7), Chapter: "Chapter 4"}
	bare := State{V: 1, State: StateArcade}
	cases := []struct {
		name, tmpl string
		in         State
		want       string
	}{
		{"placeholders", "{title} - {artist} ({charter}) {difficulty} {level} {pb} {rank} {stars} {chapter}", full,
			"RE-TUNED - peak divide (re-kouned) UNBEATABLE 20 1,155,577 B 8.700 Chapter 4"},
		{"missing title outside a group", "{title}", bare, "Unknown song"},
		{"missing title in a group", "[{title}|nothing]", bare, "nothing"},
		{"group kept", "{title}[ - {artist}]", full, "RE-TUNED - peak divide"},
		{"group dropped", "{title}[ - {artist}]", State{Song: "Low"}, "Low"},
		{"first complete alternative", "[{pb}|{stars}|none]", State{Stars: ptr(1.0)}, "1.000"},
		{"last alternative", "[{pb}|{stars}|none]", bare, "none"},
		{"nested group doesn't count", "[PB {pb}[ ({rank})]]", State{HighScore: ptr(5)}, "PB 5"},
		{"escapes", `\[{difficulty}\] \{x\} a\|b \\`, full, `[UNBEATABLE] {x} a|b \`},
		{"bar outside a group", "a | b", bare, "a | b"},
		{"trimmed", "[{artist} ]{title}", State{Song: "Low"}, "Low"},
		{"zero level is missing", "[Lv {level}]", State{Level: ptr(0)}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.in.State = StateArcade
			got, _ := lines(t, CustomStyle{Song: Template{Details: c.tmpl}}, c.in)
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestTemplateErrors(t *testing.T) {
	cases := map[string]string{
		"{titel}":   "unknown placeholder {titel}",
		"{title":    "isn't closed",
		"[{title}":  "isn't closed",
		"a]":        "stray ']'",
		"a}":        "stray }",
		`a\`:        `a \ at the end`,
		"[a|[b|c]":  "isn't closed",
		"x {pb} {}": "unknown placeholder {}",
	}
	for tmpl, want := range cases {
		_, err := CustomStyle{Story: Template{State: tmpl}}.Compile()
		if err == nil || !strings.Contains(err.Error(), want) || !strings.HasPrefix(err.Error(), "Story, second line: ") {
			t.Errorf("%q: got %v, want an error about %q", tmpl, err, want)
		}
	}
}

func TestTemplateFallbacks(t *testing.T) {
	c := CustomStyle{Song: Template{"song {title}", "x"}}
	cases := []struct {
		in             State
		details, state string
	}{
		{State{State: StateArcade, Song: "A"}, "song A", "x"},
		{State{State: StateResults, Song: "A"}, "song A", "x"},
		{State{State: StateStory, Chapter: "Chapter 1"}, "Story mode - Chapter 1", ""},
		{State{State: StateStory, Chapter: "Chapter 1", Song: "A"}, "Story mode - Chapter 1", ""},
		{State{State: StateIdle, Stars: ptr(2.0)}, "In menus", "★ 2.000"},
	}
	for _, tc := range cases {
		if d, s := lines(t, c, tc.in); d != tc.details || s != tc.state {
			t.Errorf("%s: got %q / %q, want %q / %q", tc.in.State, d, s, tc.details, tc.state)
		}
	}

	c.Results = Template{Details: "done"}
	c.Story = Template{Details: "walking"}
	c.StorySong = Template{State: "story {title}"}
	c.Menus = Template{State: "menus"}
	for _, tc := range []struct {
		in             State
		details, state string
	}{
		{State{State: StateResults, Song: "A"}, "done", ""},
		{State{State: StateStory}, "walking", ""},
		{State{State: StateStory, Song: "A"}, "", "story A"},
		{State{State: StateIdle}, "", "menus"},
	} {
		if d, s := lines(t, c, tc.in); d != tc.details || s != tc.state {
			t.Errorf("%s: got %q / %q, want %q / %q", tc.in.State, d, s, tc.details, tc.state)
		}
	}
}

// Duplicating a built-in in the app must start from something that looks the same.
func TestBuiltinTemplatesMatchBuiltins(t *testing.T) {
	var states []State
	for _, st := range []string{StateArcade, StateResults, StateStory, StateIdle} {
		for _, song := range []string{"", "RE-TUNED"} {
			for _, artist := range []string{"", "peak divide"} {
				for _, chart := range []State{{}, {Difficulty: "Hard"}, {Level: ptr(3)}, {Difficulty: "UNBEATABLE", Level: ptr(20), Charter: "c"}} {
					for _, score := range []State{{}, {HighScore: ptr(1155577)}, {HighScore: ptr(9), Rank: "S"}} {
						for _, extra := range []State{{}, {Chapter: "Chapter 4", Stars: ptr(8.703)}} {
							states = append(states, State{V: 1, State: st, Song: song, Artist: artist,
								Difficulty: chart.Difficulty, Level: chart.Level, Charter: chart.Charter,
								HighScore: score.HighScore, Rank: score.Rank, Chapter: extra.Chapter, Stars: extra.Stars})
						}
					}
				}
			}
		}
	}
	for _, b := range Styles {
		tmpl, err := b.Template.Compile()
		if err != nil {
			t.Fatalf("%s: %v", b.Name, err)
		}
		for _, s := range states {
			wd, ws := b.Style(s)
			gd, gs := tmpl(s)
			if gd != wd || gs != ws {
				t.Errorf("%s, %+v: template gives %q / %q, style gives %q / %q", b.Name, s, gd, gs, wd, ws)
			}
		}
	}
}

func TestValidateCustom(t *testing.T) {
	ok := CustomStyle{Name: "mine", Song: Template{Details: "{title}"}}
	if err := ValidateCustom([]CustomStyle{ok, {Name: "Other"}}); err != nil {
		t.Errorf("valid styles: %v", err)
	}
	bad := map[string][]CustomStyle{
		"no name":         {{Name: " "}},
		"spaces":          {{Name: "mine "}},
		"too long":        {{Name: strings.Repeat("x", maxStyleName+1)}},
		"built-in":        {{Name: "Spotify"}},
		"duplicate":       {ok, {Name: "MINE"}},
		"broken template": {{Name: "x", Menus: Template{Details: "{nope}"}}},
	}
	for name, styles := range bad {
		if err := ValidateCustom(styles); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLookupCustomStyle(t *testing.T) {
	custom := []CustomStyle{{Name: "Mine", Song: Template{Details: "♪ {title}"}}, {Name: "broken", Song: Template{Details: "{"}}}
	style, err := LookupStyle("mine", custom)
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := style(State{State: StateArcade, Song: "Low"}); d != "♪ Low" {
		t.Errorf("got %q", d)
	}
	if _, err := LookupStyle("broken", custom); err == nil {
		t.Error("broken custom style looked up")
	}
	if _, err := LookupStyle("nope", custom); err == nil || !strings.Contains(err.Error(), "Mine") {
		t.Errorf("unknown style error should list custom styles: %v", err)
	}
}
