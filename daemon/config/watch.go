package config

import (
	"context"
	"os"
	"time"
)

// Watch calls changed whenever the file at path appears, disappears, or
// changes size or modification time, until ctx is done. It polls, so it needs
// nothing outside the standard library and works the same on every OS.
func Watch(ctx context.Context, path string, every time.Duration, changed func()) {
	last := stamp(path)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s := stamp(path); s != last {
				last = s
				changed()
			}
		}
	}
}

type fileStamp struct {
	exists bool
	size   int64
	mod    time.Time
}

func stamp(path string) fileStamp {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{exists: true, size: fi.Size(), mod: fi.ModTime()}
}
