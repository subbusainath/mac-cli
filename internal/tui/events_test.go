package tui

import (
	"encoding/json"
	"testing"
)

func TestDecodeAwaitApproval(t *testing.T) {
	line := `{"event":"await_approval","changes":[{"path":"a.py","old":"x","new":"y"}]}`
	var ev CodeEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatal(err)
	}
	if ev.Event != "await_approval" || len(ev.Changes) != 1 || ev.Changes[0].Path != "a.py" {
		t.Fatalf("decoded %+v", ev)
	}
}

func TestEncodeCommand(t *testing.T) {
	b, err := json.Marshal(CodeCommand{Cmd: "feedback", Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"cmd":"feedback","text":"hi"}` {
		t.Fatalf("got %s", b)
	}
}
