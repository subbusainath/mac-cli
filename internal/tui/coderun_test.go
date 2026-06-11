package tui

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestRun() *codeRunModel {
	m := newCodeRunModel("demo task")
	var buf bytes.Buffer
	m.cmdWriter = &buf
	return m
}

func TestAwaitApprovalSwitchesMode(t *testing.T) {
	m := newTestRun()
	mm, _ := m.Update(eventMsg{CodeEvent{Event: "await_approval",
		Changes: []FileDelta{{Path: "a.py", Old: "", New: "pass\n"}}}})
	m = mm.(*codeRunModel)
	if m.mode != modeApproval {
		t.Fatalf("mode = %v", m.mode)
	}
	if !strings.Contains(m.View(), "a.py") {
		t.Fatal("approval view must list the file")
	}
}

func TestApproveKeySendsCommand(t *testing.T) {
	m := newTestRun()
	mm, _ := m.Update(eventMsg{CodeEvent{Event: "await_approval",
		Changes: []FileDelta{{Path: "a.py", New: "pass\n"}}}})
	m = mm.(*codeRunModel)
	buf := m.cmdWriter.(*bytes.Buffer)
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = mm.(*codeRunModel)
	if !strings.Contains(buf.String(), `"cmd":"approve"`) {
		t.Fatalf("stdin got %q", buf.String())
	}
	if m.mode != modeRunning {
		t.Fatalf("mode = %v", m.mode)
	}
}

func TestPhaseEventUpdatesTracker(t *testing.T) {
	m := newTestRun()
	mm, _ := m.Update(eventMsg{CodeEvent{Event: "phase", Phase: "GREEN", Iterations: 3}})
	m = mm.(*codeRunModel)
	if m.phase != "GREEN" || m.iterations != 3 {
		t.Fatalf("phase=%q iter=%d", m.phase, m.iterations)
	}
}

func TestDoneEventQuits(t *testing.T) {
	m := newTestRun()
	mm, cmd := m.Update(eventMsg{CodeEvent{Event: "done", Iterations: 4}})
	m = mm.(*codeRunModel)
	if !m.finished || cmd == nil {
		t.Fatal("done must finish the program")
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestRenderUnifiedDiffColors(t *testing.T) {
	out := ansiRe.ReplaceAllString(renderUnifiedDiff("a.py", "x = 1\n", "x = 2\n", 80), "")
	if !strings.Contains(out, "x = 1") || !strings.Contains(out, "x = 2") {
		t.Fatalf("diff missing lines:\n%s", out)
	}
}
