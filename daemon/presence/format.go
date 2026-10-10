package presence

import (
	"unicode/utf8"

	"github.com/decbr/better-unbeatable-rpc/daemon/internal/discordipc"
)

type Options struct {
	LargeImage string
	LargeText  string
	Style      Style // see style.go :)
}

func Format(s State, o Options) *discordipc.Activity {
	if s.State == StateClosed {
		return nil
	}
	style := o.Style
	if style == nil {
		style = Detailed
	}

	a := &discordipc.Activity{}
	a.Details, a.State = style(s)

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
	}
	return a
}

func songTitle(s State) string {
	if s.Song == "" {
		return "Unknown song"
	}
	return s.Song
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
