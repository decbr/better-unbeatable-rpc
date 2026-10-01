package presence

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/decbr/better-unbeatable-rpc/daemon/internal/discordipc"
)

type Options struct {
	LargeImage string
	LargeText  string
}

func Format(s State, o Options) *discordipc.Activity {
	a := &discordipc.Activity{}

	switch s.State {
	case StateArcade:
		a.Details = songLine(s)
		a.State = pbLine(s)
	case StateResults:
		a.Details = "Results: " + songLine(s)
		a.State = pbLine(s)
	case StateStory:
		a.Details = "Story mode"
		if s.Chapter != "" {
			a.Details += " - " + s.Chapter
		}
		a.State = starsLine(s)
	case StateIdle:
		a.Details = "In menus"
		a.State = starsLine(s)
	default:
		return nil
	}

	inSong := s.State == StateArcade || s.State == StateStory && s.Song != ""
	if inSong && s.Paused {
		a.Details += " (paused)"
	} else if inSong && s.StartedAt > 0 {
		a.Timestamps = &discordipc.Timestamps{Start: s.StartedAt}
	}

	a.Details = fitField(a.Details)
	a.State = fitField(a.State)

	if o.LargeImage != "" {
		a.Assets = &discordipc.Assets{LargeImage: o.LargeImage, LargeText: o.LargeText}
		if (s.State == StateArcade || s.State == StateResults) && s.Artist != "" {
			a.Assets.LargeText = fitField(s.Artist)
		}
	}
	return a
}

func songLine(s State) string {
	song := s.Song
	if song == "" {
		song = "Unknown song"
	}
	var diff []string
	if s.Difficulty != "" {
		diff = append(diff, s.Difficulty)
	}
	if s.Level != nil && *s.Level > 0 {
		diff = append(diff, fmt.Sprintf("Lv %d", *s.Level))
	}
	if len(diff) == 0 {
		return song
	}
	return song + " [" + strings.Join(diff, " ") + "]"
}

func starsLine(s State) string {
	if s.Stars == nil {
		return ""
	}
	return "★ " + strconv.FormatFloat(*s.Stars, 'f', 3, 64)
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

const (
	minField = 2
	maxField = 128
)

func fitField(s string) string {
	if s == "" {
		return ""
	}
	if n := utf8.RuneCountInString(s); n < minField {
		return s + " "
	} else if n > maxField {
		r := []rune(s)
		return string(r[:maxField-1]) + "…"
	}
	return s
}
