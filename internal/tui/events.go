package tui

// CodeEvent is one line of the orchestrator's JSON protocol (--ui json).
type CodeEvent struct {
	Event      string      `json:"event"`
	Project    string      `json:"project,omitempty"`
	SessionID  string      `json:"session_id,omitempty"`
	Task       string      `json:"task,omitempty"`
	Node       string      `json:"node,omitempty"`
	Step       int         `json:"step,omitempty"`
	Text       string      `json:"text,omitempty"`
	Phase      string      `json:"phase,omitempty"`
	Verdict    string      `json:"verdict,omitempty"`
	Iterations int         `json:"iterations,omitempty"`
	ExitCode   int         `json:"exit_code,omitempty"`
	Tail       string      `json:"tail,omitempty"`
	Changes    []FileDelta `json:"changes,omitempty"`
	Reason     string      `json:"reason,omitempty"`
	Message    string      `json:"message,omitempty"`
}

// FileDelta carries full old/new contents; the Go side renders the diff.
type FileDelta struct {
	Path string `json:"path"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

// CodeCommand is sent to the orchestrator's stdin.
type CodeCommand struct {
	Cmd  string `json:"cmd"`
	Text string `json:"text,omitempty"`
}
