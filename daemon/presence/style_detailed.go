package presence

import (
	"fmt"
	"strconv"
	"strings"
)

//	arcade   RE-TUNED - peak divide      	 / UNBEATABLE Lv 20 · PB: 1,155,577 (B)
//	results  Results: RE-TUNED - peak divide / UNBEATABLE Lv 20 · PB: 1,155,577 (B)
//	story    Story mode - Chapter 4       	 / ★ 8.703
//	menus    In menus                    	 / ★ 8.703

func Detailed(s State) (details, state string) {
	switch s.State {
	case StateArcade:
		return songLine(s), chartLine(s)
	case StateResults:
		return "Results: " + songLine(s), chartLine(s)
	case StateStory:
		details = "Story mode"
		if s.Chapter != "" {
			details += " - " + s.Chapter
		}
		return details, starsLine(s)
	default:
		return "In menus", starsLine(s)
	}
}

func songLine(s State) string {
	song := songTitle(s)
	if s.Artist == "" {
		return song
	}
	return song + " - " + s.Artist
}

func chartLine(s State) string {
	var parts []string
	if s.Difficulty != "" {
		parts = append(parts, s.Difficulty)
	}
	if s.Level != nil && *s.Level > 0 {
		parts = append(parts, fmt.Sprintf("Lv %d", *s.Level))
	}
	chart := strings.Join(parts, " ")
	if chart == "" {
		return pbLine(s)
	}
	return chart + " · " + pbLine(s)
}

func pbLine(s State) string {
	if s.HighScore == nil {
		return "No PB yet"
	}
	line := "PB: " + thousands(*s.HighScore)
	if s.Rank != "" {
		line += " (" + s.Rank + ")"
	}
	return line
}

func thousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

var detailedTemplate = CustomStyle{
	Name:    "detailed",
	Song:    Template{"{title}[ - {artist}]", detailedChart},
	Results: Template{"Results: {title}[ - {artist}]", detailedChart},
	Story:   Template{"Story mode[ - {chapter}]", "[★ {stars}]"},
	Menus:   Template{"In menus", "[★ {stars}]"},
}

const detailedChart = "[{difficulty}[ Lv {level}] · |Lv {level} · ][PB: {pb}[ ({rank})]|No PB yet]"
