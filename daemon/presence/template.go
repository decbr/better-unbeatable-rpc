package presence

import (
	"fmt"
	"strconv"
	"strings"
)

//	{title}                     a placeholder
//	[ - {artist}]               left out if a placeholder directly in it is empty
//	[PB: {pb}|No PB yet]        the first alternative with all its placeholders
//	\[ \] \{ \} \| \\           literal characters
//

type Template struct {
	Details string `json:"details,omitempty"`
	State   string `json:"state,omitempty"`
}

func (t Template) empty() bool { return t.Details == "" && t.State == "" }

type CustomStyle struct {
	Name      string   `json:"name"`
	Song      Template `json:"song"`
	Results   Template `json:"results"`
	StorySong Template `json:"storySong"` // a rhythm section in story mode
	Story     Template `json:"story"`
	Menus     Template `json:"menus"`
}

var Placeholders = []string{"title", "artist", "charter", "difficulty", "level", "pb", "rank", "stars", "chapter"}

func placeholder(s State, name string) (string, bool) {
	switch name {
	case "title":
		return songTitle(s), s.Song != ""
	case "artist":
		return s.Artist, s.Artist != ""
	case "charter":
		return s.Charter, s.Charter != ""
	case "difficulty":
		return s.Difficulty, s.Difficulty != ""
	case "level":
		if s.Level == nil || *s.Level <= 0 {
			return "", false
		}
		return strconv.Itoa(*s.Level), true
	case "pb":
		if s.HighScore == nil {
			return "", false
		}
		return thousands(*s.HighScore), true
	case "rank":
		return s.Rank, s.Rank != ""
	case "stars":
		if s.Stars == nil {
			return "", false
		}
		return strconv.FormatFloat(*s.Stars, 'f', 3, 64), true
	case "chapter":
		return s.Chapter, s.Chapter != ""
	}
	return "", false
}

func isPlaceholder(name string) bool {
	for _, p := range Placeholders {
		if p == name {
			return true
		}
	}
	return false
}

// node is literal text, a placeholder, or a group of alternatives.
type node struct {
	text  string
	field string
	alts  [][]node
}

// parseTemplate turns a template line into nodes.
func parseTemplate(src string) ([]node, error) {
	p := &parser{src: []rune(src)}
	nodes, err := p.seq(0)
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.src) {
		return nil, fmt.Errorf("stray %q at %d; write \\%c for the character itself", p.src[p.pos], p.pos+1, p.src[p.pos])
	}
	return nodes, nil
}

type parser struct {
	src []rune
	pos int
}

// seq reads until the end, or a ] or | that closes the group it's in.
func (p *parser) seq(depth int) ([]node, error) {
	var nodes []node
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			nodes = append(nodes, node{text: text.String()})
			text.Reset()
		}
	}
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		switch r {
		case '\\':
			if p.pos+1 == len(p.src) {
				return nil, fmt.Errorf(`a \ at the end; write \\ for the character itself`)
			}
			text.WriteRune(p.src[p.pos+1])
			p.pos += 2
		case '{':
			end := p.pos + 1
			for end < len(p.src) && p.src[end] != '}' {
				end++
			}
			if end == len(p.src) {
				return nil, fmt.Errorf("{ at %d isn't closed", p.pos+1)
			}
			name := string(p.src[p.pos+1 : end])
			if !isPlaceholder(name) {
				return nil, fmt.Errorf("unknown placeholder {%s} (have: %s)", name, strings.Join(Placeholders, ", "))
			}
			flush()
			nodes = append(nodes, node{field: name})
			p.pos = end + 1
		case '[':
			flush()
			start := p.pos
			p.pos++
			var alts [][]node
			for {
				alt, err := p.seq(depth + 1)
				if err != nil {
					return nil, err
				}
				alts = append(alts, alt)
				if p.pos == len(p.src) {
					return nil, fmt.Errorf("[ at %d isn't closed", start+1)
				}
				closer := p.src[p.pos]
				p.pos++
				if closer == ']' {
					break
				}
			}
			nodes = append(nodes, node{alts: alts})
		case ']', '|':
			if depth == 0 && r == '|' {
				text.WriteRune(r)
				p.pos++
				continue
			}
			flush()
			return nodes, nil
		case '}':
			return nil, fmt.Errorf("stray } at %d; write \\} for the character itself", p.pos+1)
		default:
			text.WriteRune(r)
			p.pos++
		}
	}
	flush()
	return nodes, nil
}

func render(nodes []node, s State) string {
	var b strings.Builder
	for _, n := range nodes {
		switch {
		case n.field != "":
			v, _ := placeholder(s, n.field)
			b.WriteString(v)
		case n.alts != nil:
			for _, alt := range n.alts {
				if complete(alt, s) {
					b.WriteString(render(alt, s))
					break
				}
			}
		default:
			b.WriteString(n.text)
		}
	}
	return b.String()
}

// complete reports whether every placeholder directly in nodes has a value.
func complete(nodes []node, s State) bool {
	for _, n := range nodes {
		if n.field != "" {
			if _, ok := placeholder(s, n.field); !ok {
				return false
			}
		}
	}
	return true
}

type compiled struct {
	details, state []node
	set            bool
}

func compileTemplate(t Template, part string) (compiled, error) {
	details, err := parseTemplate(t.Details)
	if err != nil {
		return compiled{}, fmt.Errorf("%s, first line: %w", part, err)
	}
	state, err := parseTemplate(t.State)
	if err != nil {
		return compiled{}, fmt.Errorf("%s, second line: %w", part, err)
	}
	return compiled{details, state, !t.empty()}, nil
}

func (c compiled) lines(s State) (details, state string) {
	return strings.TrimSpace(render(c.details, s)), strings.TrimSpace(render(c.state, s))
}

// Compile checks the templates and makes the style. The name isn't checked:
// see ValidateCustom.
func (c CustomStyle) Compile() (Style, error) {
	var parts [5]compiled
	for i, t := range []struct {
		t    Template
		name string
	}{{c.Song, "Song"}, {c.Results, "Results"}, {c.StorySong, "Story song"}, {c.Story, "Story"}, {c.Menus, "Menus"}} {
		var err error
		if parts[i], err = compileTemplate(t.t, t.name); err != nil {
			return nil, err
		}
	}
	song, results, storySong, story, menus := parts[0], parts[1], parts[2], parts[3], parts[4]

	return func(s State) (string, string) {
		pick := song
		switch {
		case s.State == StateArcade:
		case s.State == StateResults:
			if results.set {
				pick = results
			}
		case s.State == StateStory && s.Song != "" && storySong.set:
			pick = storySong
		case s.State == StateStory:
			if !story.set {
				return Detailed(s)
			}
			pick = story
		default:
			if !menus.set {
				return Detailed(s)
			}
			pick = menus
		}
		return pick.lines(s)
	}, nil
}

const maxStyleName = 40

// ValidateCustom checks custom styles can be saved: each has a name that's
// unique and not a built-in's, and its templates parse.
func ValidateCustom(styles []CustomStyle) error {
	seen := map[string]bool{}
	for _, c := range styles {
		name := strings.ToLower(c.Name)
		switch {
		case strings.TrimSpace(c.Name) == "":
			return fmt.Errorf("a custom style has no name")
		case c.Name != strings.TrimSpace(c.Name):
			return fmt.Errorf("style name %q starts or ends with a space", c.Name)
		case len([]rune(c.Name)) > maxStyleName:
			return fmt.Errorf("style name %q is longer than %d characters", c.Name, maxStyleName)
		case builtin(name) != nil:
			return fmt.Errorf("%q is a built-in style's name", c.Name)
		case seen[name]:
			return fmt.Errorf("two custom styles are called %q", c.Name)
		}
		seen[name] = true
		if _, err := c.Compile(); err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
	}
	return nil
}
