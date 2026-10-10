package presence

// song     RE-TUNED   / peak divide ft. Rachel Lake
// story    Story mode / Chapter 4
// menus    In menus   /

func Spotify(s State) (details, state string) {
	switch {
	case s.State == StateArcade, s.State == StateResults, s.State == StateStory && s.Song != "":
		return songTitle(s), s.Artist
	case s.State == StateStory:
		return "Story mode", s.Chapter
	default:
		return "In menus", starsLine(s)
	}
}
