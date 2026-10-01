package discordipc

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var subdirs = []string{
	"",
	"app/com.discordapp.Discord",
	"app/dev.vencord.Vesktop",
	".flatpak/dev.vencord.Vesktop/xdg-run",
	".flatpak/com.discordapp.Discord/xdg-run",
	"snap.discord",
}

func CandidatePaths() []string {
	var bases []string
	seenBase := map[string]bool{}
	for _, env := range []string{"XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP"} {
		if v := os.Getenv(env); v != "" && !seenBase[v] {
			bases = append(bases, v)
			seenBase[v] = true
		}
	}
	if !seenBase["/tmp"] {
		bases = append(bases, "/tmp")
	}

	var paths []string
	for _, base := range bases {
		for _, sub := range subdirs {
			for i := 0; i < 10; i++ {
				p := filepath.Join(base, sub, fmt.Sprintf("discord-ipc-%d", i))
				if fi, err := os.Stat(p); err == nil && fi.Mode().Type() == fs.ModeSocket {
					paths = append(paths, p)
				}
			}
		}
	}
	return paths
}
