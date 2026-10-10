package presence

// song     peak divide - RE-TUNED (charter) [UNBEATABLE] / Doing crimes
// story    Story mode / Chapter 4
// menus    In menus   / ★ 8.703

// osuActivity is the second line while playing, like osu!'s "Clicking circles".
const osuActivity = "Doing crimes"

func Osu(s State) (details, state string) {
	switch {
	case s.State == StateArcade, s.State == StateResults, s.State == StateStory && s.Song != "":
		return osuLine(s), osuActivity
	case s.State == StateStory:
		return "Story mode", s.Chapter
	default:
		return "In menus", starsLine(s)
	}
}

// osuLine is osu!'s "Artist - Song (Mapper) [Difficulty]", leaving out what's missing.
func osuLine(s State) string {
	line := songTitle(s)
	if s.Artist != "" {
		line = s.Artist + " - " + line
	}
	if s.Charter != "" {
		line += " (" + s.Charter + ")"
	}
	if s.Difficulty != "" {
		line += " [" + s.Difficulty + "]"
	}
	return line
}
