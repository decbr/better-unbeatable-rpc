package presence

import (
	"fmt"
	"strconv"
	"strings"
)

type Style func(s State) (details, state string)

const DefaultStyle = "detailed"

type NamedStyle struct {
	Name  string
	Style Style
	// Template is the style written as a custom one, the starting point when
	// it's duplicated in the settings app.
	Template CustomStyle
}

var Styles = []NamedStyle{
	{"detailed", Detailed, detailedTemplate},
	{"spotify", Spotify, spotifyTemplate},
	{"osu", Osu, osuTemplate},
	// add more...
}

func StyleNames() []string {
	names := make([]string, len(Styles))
	for i, s := range Styles {
		names[i] = s.Name
	}
	return names
}

func builtin(name string) Style {
	for _, s := range Styles {
		if s.Name == strings.ToLower(name) {
			return s.Style
		}
	}
	return nil
}

// LookupStyle finds a built-in style, or one of custom, by name.
func LookupStyle(name string, custom []CustomStyle) (Style, error) {
	if s := builtin(name); s != nil {
		return s, nil
	}
	names := StyleNames()
	for _, c := range custom {
		if strings.EqualFold(c.Name, name) {
			s, err := c.Compile()
			if err != nil {
				return nil, fmt.Errorf("style %q: %w", c.Name, err)
			}
			return s, nil
		}
		names = append(names, c.Name)
	}
	return nil, fmt.Errorf("unknown style %q (have: %s)", name, strings.Join(names, ", "))
}

func starsLine(s State) string {
	if s.Stars == nil {
		return ""
	}
	return "★ " + strconv.FormatFloat(*s.Stars, 'f', 3, 64)
}
