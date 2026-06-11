package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subbusainath/mac-cli/internal/db"
)

// ── Delegate ──────────────────────────────────────────────────────────────

type projectItem struct{ project *db.Project }

func (i projectItem) FilterValue() string {
	if i.project == nil {
		return "New Project"
	}
	return i.project.Name
}

type projectDelegate struct{}

func (d projectDelegate) Height() int                             { return 2 }
func (d projectDelegate) Spacing() int                            { return 0 }
func (d projectDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d projectDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	pi, ok := item.(projectItem)
	if !ok {
		return
	}

	selected := index == m.Index()

	if pi.project == nil {
		// "New Project" option with a sparkle icon
		label := lipgloss.NewStyle().Bold(true).Render("✦  New Project")
		hint := DimStyle("  create a new project from scratch")
		if selected {
			fmt.Fprint(w,
				lipgloss.NewStyle().
					BorderStyle(lipgloss.ThickBorder()).
					BorderLeft(true).
					BorderForeground(clrPurple).
					PaddingLeft(1).
					Render(
						lipgloss.JoinHorizontal(lipgloss.Top,
							AccentStyle("▶  "+label),
							DimStyle("  create a new project"),
						),
					),
			)
		} else {
			fmt.Fprint(w,
				lipgloss.NewStyle().PaddingLeft(4).Render(label+hint),
			)
		}
		return
	}

	name := pi.project.Name

	if selected {
		rendered := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(clrPurple).Render("▶  "+name),
			lipgloss.NewStyle().PaddingLeft(4).Foreground(clrMuted).Render(pi.project.Path),
		)
		fmt.Fprint(w,
			lipgloss.NewStyle().
				BorderStyle(lipgloss.ThickBorder()).
				BorderLeft(true).
				BorderForeground(clrPurple).
				PaddingLeft(1).
				Render(rendered),
		)
	} else {
		rendered := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().PaddingLeft(4).Render(name),
			lipgloss.NewStyle().PaddingLeft(4).Foreground(clrMuted).Render(pi.project.Path),
		)
		fmt.Fprint(w, rendered)
	}
}

// ── Model ─────────────────────────────────────────────────────────────────

type pickerModel struct {
	list     list.Model
	choice   *db.Project
	quitting bool
	width    int
	height   int
}

func newPickerModel(projects []db.Project) pickerModel {
	items := []list.Item{projectItem{nil}}
	for i := range projects {
		items = append(items, projectItem{&projects[i]})
	}

	// ── custom delegate ─────────────────────────────────────────────────
	d := projectDelegate{}

	l := list.New(items, d, 70, 18)
	l.Title = "select a project"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.DisableQuitKeybindings()

	// ── Style the list chrome ───────────────────────────────────────────
	sty := list.DefaultStyles()
	sty.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(clrPurple).
		PaddingLeft(2)
	sty.FilterPrompt = lipgloss.NewStyle().
		Foreground(clrCyan).
		Bold(true).
		PaddingLeft(2)
	sty.FilterCursor = lipgloss.NewStyle().
		Foreground(clrPurple)
	sty.PaginationStyle = lipgloss.NewStyle().
		Foreground(clrMuted).
		PaddingLeft(2)
	sty.HelpStyle = lipgloss.NewStyle().
		Foreground(clrBorder).
		PaddingLeft(2).
		PaddingBottom(1)
	sty.StatusBar = lipgloss.NewStyle().
		Foreground(clrMuted).
		PaddingLeft(2)
	l.Styles = sty

	return pickerModel{list: l, width: 70, height: 18}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			selected, ok := m.list.SelectedItem().(projectItem)
			if ok {
				m.choice = selected.project
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width - 4)
		m.list.SetHeight(msg.Height - 4)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m pickerModel) View() string {
	var b strings.Builder

	// ── Banner ──────────────────────────────────────────────────────────
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(clrPurple).
		Render("◆  mac")
	sub := SubtitleStyle("  project selector")
	sep := DimStyle(strings.Repeat("─", max(0, m.width-lipgloss.Width(banner+sub)-4)))
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, banner, sub, " ", sep))
	b.WriteString("\n\n")

	// ── List ────────────────────────────────────────────────────────────
	b.WriteString(m.list.View())

	return b.String()
}
