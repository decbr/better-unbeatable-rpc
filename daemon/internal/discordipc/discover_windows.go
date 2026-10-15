package discordipc

import "fmt"

// CandidatePaths are Discord's pipes, unchecked: opening one would use up an instance.
func CandidatePaths() []string {
	paths := make([]string, 10)
	for i := range paths {
		paths[i] = fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
	}
	return paths
}
