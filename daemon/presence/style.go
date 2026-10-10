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
}

var Styles = []NamedStyle{
	{"detailed", Detailed},
	{"spotify", Spotify},
	{"osu", Osu},
	// add more...
}

func StyleNames() []string {
	names := make([]string, len(Styles))
	for i, s := range Styles {
		names[i] = s.Name
	}
	return names
}

func LookupStyle(name string) (Style, error) {
	for _, s := range Styles {
		if s.Name == strings.ToLower(name) {
			return s.Style, nil
		}
	}
	return nil, fmt.Errorf("unknown style %q (have: %s)", name, strings.Join(StyleNames(), ", "))
}

func starsLine(s State) string {
	if s.Stars == nil {
		return ""
	}
	return "★ " + strconv.FormatFloat(*s.Stars, 'f', 3, 64)
}
