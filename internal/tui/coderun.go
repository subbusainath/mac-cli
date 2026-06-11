package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/aymanbagabas/go-udiff"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type runMode int

const (
	modeRunning runMode = iota
	modeApproval
	modeFeedback
)

type eventMsg struct{ ev CodeEvent }
type procExitMsg struct{ err error }

const maxIterations = 12 // mirror of routing.MAX_ITERATIONS for display

type codeRunModel struct {
	task       string
	sessionID  string
	mode       runMode
	phase      string
	iterations int
	finished   bool
	exitErr    string

	activity viewport.Model
	logLines []string
	stream   strings.Builder // current token stream

	changes  []FileDelta
	fileIdx  int
	diffView viewport.Model
	feedback textarea.Model

	cmdWriter io.Writer // orchestrator stdin (a *bytes.Buffer in tests)
	width     int
	height    int
}

func newCodeRunModel(task string) *codeRunModel {
	fb := textarea.New()
	fb.Placeholder = "what should change?"
	return &codeRunModel{
		task:     task,
		activity: viewport.New(76, 10),
		diffView: viewport.New(76, 12),
		feedback: fb,
		width:    80, height: 24,
	}
}

func (m *codeRunModel) Init() tea.Cmd { return nil }

func (m *codeRunModel) send(c CodeCommand) {
	if m.cmdWriter == nil {
		return
	}
	b, _ := json.Marshal(c)
	fmt.Fprintln(m.cmdWriter, string(b))
}

func (m *codeRunModel) log(line string) {
	m.logLines = append(m.logLines, line)
	if len(m.logLines) > 200 {
		m.logLines = m.logLines[len(m.logLines)-200:]
	}
	m.activity.SetContent(strings.Join(m.logLines, "\n"))
	m.activity.GotoBottom()
}

func (m *codeRunModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.activity.Width = msg.Width - 4
		m.diffView.Width = msg.Width - 30
		return m, nil

	case eventMsg:
		return m.handleEvent(msg.ev)

	case procExitMsg:
		m.finished = true
		if msg.err != nil && m.exitErr == "" {
			m.exitErr = msg.err.Error()
		}
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *codeRunModel) handleEvent(ev CodeEvent) (tea.Model, tea.Cmd) {
	switch ev.Event {
	case "session":
		m.sessionID = ev.SessionID
	case "token":
		m.stream.WriteString(ev.Text)
		// show last partial line of the stream as live activity
		tail := m.stream.String()
		if i := strings.LastIndexByte(tail, '\n'); i >= 0 {
			tail = tail[i+1:]
		}
		if len(tail) > 60 {
			tail = tail[len(tail)-60:]
		}
		m.replaceOrAppendStreamLine(fmt.Sprintf("▸ %s streaming… %q", ev.Node, tail))
	case "node_end":
		m.stream.Reset()
		m.log(fmt.Sprintf("▸ %s done (step %d)", ev.Node, ev.Step))
	case "phase":
		m.phase = ev.Phase
		m.iterations = ev.Iterations
		m.log(fmt.Sprintf("▸ phase %s · verdict %s", ev.Phase, ev.Verdict))
	case "test_output":
		status := SuccessStyle("PASS")
		if ev.ExitCode != 0 {
			status = ErrorStyle(fmt.Sprintf("FAIL (exit %d)", ev.ExitCode))
		}
		m.log("▸ tests: " + status)
	case "await_approval":
		m.changes = ev.Changes
		m.fileIdx = 0
		m.mode = modeApproval
		m.refreshDiff()
	case "done":
		m.log(SuccessStyle(fmt.Sprintf("✓ done — green after %d test runs", ev.Iterations)))
		m.finished = true
		return m, tea.Quit
	case "halt":
		m.exitErr = ev.Reason
		m.finished = true
		return m, tea.Quit
	case "error":
		m.exitErr = ev.Message
		m.finished = true
		return m, tea.Quit
	}
	return m, nil
}

// replaceOrAppendStreamLine keeps exactly one live "streaming…" line at the tail.
func (m *codeRunModel) replaceOrAppendStreamLine(line string) {
	if n := len(m.logLines); n > 0 && strings.Contains(m.logLines[n-1], "streaming…") {
		m.logLines[n-1] = line
	} else {
		m.logLines = append(m.logLines, line)
	}
	m.activity.SetContent(strings.Join(m.logLines, "\n"))
	m.activity.GotoBottom()
}

func (m *codeRunModel) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeFeedback {
		switch key.Type {
		case tea.KeyEsc:
			m.mode = modeApproval
			return m, nil
		case tea.KeyCtrlD: // submit feedback
			m.send(CodeCommand{Cmd: "feedback", Text: m.feedback.Value()})
			m.feedback.Reset()
			m.mode = modeRunning
			return m, nil
		}
		var cmd tea.Cmd
		m.feedback, cmd = m.feedback.Update(key)
		return m, cmd
	}

	switch key.String() {
	case "ctrl+c", "q":
		m.send(CodeCommand{Cmd: "quit"})
		m.finished = true
		return m, tea.Quit
	}

	if m.mode == modeApproval {
		switch key.String() {
		case "a":
			m.send(CodeCommand{Cmd: "approve"})
			m.mode = modeRunning
			m.log("▸ approved — applying")
		case "f":
			m.mode = modeFeedback
			m.feedback.Focus()
		case "tab":
			if len(m.changes) > 0 {
				m.fileIdx = (m.fileIdx + 1) % len(m.changes)
				m.refreshDiff()
			}
		case "j", "down":
			m.diffView.LineDown(1)
		case "k", "up":
			m.diffView.LineUp(1)
		}
	}
	return m, nil
}

func (m *codeRunModel) refreshDiff() {
	if m.fileIdx >= len(m.changes) {
		return
	}
	c := m.changes[m.fileIdx]
	m.diffView.SetContent(renderUnifiedDiff(c.Path, c.Old, c.New, m.diffView.Width))
	m.diffView.GotoTop()
}

// renderUnifiedDiff computes a unified diff and colorizes it: additions
// green (chroma-highlighted by file type), deletions red, hunks cyan.
func renderUnifiedDiff(path, oldContent, newContent string, width int) string {
	unified := udiff.Unified("a/"+path, "b/"+path, oldContent, newContent)
	var b strings.Builder
	for _, line := range strings.Split(unified, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			b.WriteString(DimStyle(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(lipgloss.NewStyle().Foreground(clrCyan).Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(lipgloss.NewStyle().Foreground(clrGreen).Render("+") +
				highlightLine(path, strings.TrimPrefix(line, "+")))
		case strings.HasPrefix(line, "-"):
			b.WriteString(lipgloss.NewStyle().Foreground(clrRed).Render(line))
		default:
			b.WriteString(DimStyle(line))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// highlightLine best-effort syntax-highlights one source line with chroma.
func highlightLine(path, line string) string {
	var out strings.Builder
	if err := quick.Highlight(&out, line, path, "terminal256", "monokai"); err != nil {
		return line
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func (m *codeRunModel) View() string {
	var b strings.Builder
	b.WriteString(AccentStyle("◆ MAC · My Agentic CLI"))
	if m.sessionID != "" {
		b.WriteString(DimStyle("   session " + short(m.sessionID)))
	}
	b.WriteString("\n" + DimStyle("Task: "+m.task) + "\n\n")
	b.WriteString(renderPhaseTracker(m.phase, m.iterations) + "\n")

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(clrBorder).Padding(0, 1)
	b.WriteString(box.Render(m.activity.View()) + "\n")

	switch m.mode {
	case modeApproval:
		b.WriteString(m.viewApproval())
	case modeFeedback:
		b.WriteString("\n" + AccentStyle("feedback (ctrl+d send · esc cancel)") + "\n")
		b.WriteString(m.feedback.View() + "\n")
	default:
		b.WriteString("\n" + HintStyle("q quit") + "\n")
	}
	return b.String()
}

func (m *codeRunModel) viewApproval() string {
	var files strings.Builder
	for i, c := range m.changes {
		marker := "  "
		style := DimStyle
		if i == m.fileIdx {
			marker = "▸ "
			style = SelectedStyle
		}
		files.WriteString(style(marker+c.Path) + "\n")
	}
	left := lipgloss.NewStyle().Width(26).Render(files.String())
	right := m.diffView.View()
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	help := HintStyle("a approve · f feedback · tab file · j/k scroll · q quit")
	return fmt.Sprintf("\n%s %d files\n%s\n%s\n",
		AccentStyle("── approval ─"), len(m.changes), body, help)
}

func renderPhaseTracker(phase string, iterations int) string {
	render := func(name string, clr lipgloss.Color) string {
		dot := "○"
		style := lipgloss.NewStyle().Foreground(clrMuted)
		if phase == name {
			dot = "●"
			style = lipgloss.NewStyle().Bold(true).Foreground(clr)
		}
		return style.Render(dot + " " + name)
	}
	return fmt.Sprintf(" %s ──── %s ──── %s        iteration %d/%d",
		render("RED", clrRed), render("GREEN", clrGreen),
		render("REFACTOR", clrBlue), iterations, maxIterations)
}

func short(s string) string {
	if len(s) > 8 {
		return s[:8] + "…"
	}
	return s
}

// RunCodeOpts configures the orchestrator subprocess.
type RunCodeOpts struct {
	Bin     string // orchestrator binary (default mac-orchestrator)
	Project string
	Task    string
	DSN     string
}

// RunCode launches the orchestrator with --ui json and drives the TUI.
func RunCode(ctx context.Context, opts RunCodeOpts) error {
	cmd := exec.CommandContext(ctx, opts.Bin,
		"--project", opts.Project, "--task", opts.Task,
		"--db", opts.DSN, "--ui", "json")
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start orchestrator: %w\n\nInstall it with: uv tool install --from ./orchestrator mac-orchestrator", err)
	}

	model := newCodeRunModel(opts.Task)
	model.cmdWriter = stdin
	prog := tea.NewProgram(model)

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			var ev CodeEvent
			if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
				continue // tolerate stray non-JSON lines
			}
			prog.Send(eventMsg{ev})
		}
		prog.Send(procExitMsg{err: cmd.Wait()})
	}()

	out, err := prog.Run()
	if err != nil {
		return err
	}
	final := out.(*codeRunModel)
	if final.exitErr != "" {
		return fmt.Errorf("%s", final.exitErr)
	}
	return nil
}
