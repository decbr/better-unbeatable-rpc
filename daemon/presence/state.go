package presence

import (
	"encoding/json"
	"fmt"
)

const SchemaVersion = 1

const (
	StateIdle    = "idle"
	StateArcade  = "arcade"
	StateStory   = "story"
	StateResults = "results"
	StateClosed  = "closed"
)

type State struct {
	V          int      `json:"v"`
	State      string   `json:"state"`
	Song       string   `json:"song,omitempty"`
	Artist     string   `json:"artist,omitempty"`
	Charter    string   `json:"charter,omitempty"`
	Difficulty string   `json:"difficulty,omitempty"`
	Level      *int     `json:"level,omitempty"`
	Chapter    string   `json:"chapter,omitempty"`
	HighScore  *int     `json:"highScore,omitempty"`
	Accuracy   *float64 `json:"accuracy,omitempty"`
	Rank       string   `json:"rank,omitempty"`
	Stars      *float64 `json:"stars,omitempty"`
	Paused     bool     `json:"paused,omitempty"`
	// LaunchedAt is when the game started, in Unix seconds: the timer shown
	// in every state.
	LaunchedAt int64 `json:"launchedAt,omitempty"`
}

func ParseState(data []byte) (State, error) {
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	if s.V != SchemaVersion {
		return State{}, fmt.Errorf("unsupported message version %d (want %d)", s.V, SchemaVersion)
	}
	switch s.State {
	case StateIdle, StateArcade, StateStory, StateResults, StateClosed:
	default:
		return State{}, fmt.Errorf("unknown state %q", s.State)
	}
	return s, nil
}
