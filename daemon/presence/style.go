package presence

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Style func(s State) (details, state string)

const DefaultStyle = "detailed"

var Styles = map[string]Style{
	"detailed": Detailed,
	"spotify":  Spotify,
	// add more...
}

func StyleNames() []string {
	names := make([]string, 0, len(Styles))
	for name := range Styles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func LookupStyle(name string) (Style, error) {
	if style, ok := Styles[strings.ToLower(name)]; ok {
		return style, nil
	}
	return nil, fmt.Errorf("unknown style %q (have: %s)", name, strings.Join(StyleNames(), ", "))
}

func starsLine(s State) string {
	if s.Stars == nil {
		return ""
	}
	return "★ " + strconv.FormatFloat(*s.Stars, 'f', 3, 64)
}
