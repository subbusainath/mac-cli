package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subbusainath/mac-cli/internal/db"
)

var (
	pickerTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).MarginLeft(2)
	pickerItemStyle     = lipgloss.NewStyle().PaddingLeft(4)
	pickerSelectedStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#7C3AED")).Bold(true)
)

// projectItem wraps a db.Project for the list component.
// A nil project means "New Project".
type projectItem struct{ project *db.Project }

func (i projectItem) FilterValue() string {
	if i.project == nil {
		return "New Project"
	}
	return i.project.Name
}

type projectDelegate struct{}

func (d projectDelegate) Height() int                              { return 1 }
func (d projectDelegate) Spacing() int                            { return 0 }
func (d projectDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d projectDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	pi, ok := item.(projectItem)
	if !ok {
		return
	}

	var label string
	if pi.project == nil {
		label = "✦  New Project"
	} else {
		dim := lipgloss.NewStyle().Faint(true).Render("  " + pi.project.Path)
		label = pi.project.Name + dim
	}

	if index == m.Index() {
		fmt.Fprint(w, pickerSelectedStyle.Render("> "+label))
	} else {
		fmt.Fprint(w, pickerItemStyle.Render(label))
	}
}

type pickerModel struct {
	list     list.Model
	choice   *db.Project // nil = new project selected
	quitting bool
}

func newPickerModel(projects []db.Project) pickerModel {
	items := []list.Item{projectItem{nil}} // "New Project" always first
	for i := range projects {
		items = append(items, projectItem{&projects[i]})
	}

	l := list.New(items, projectDelegate{}, 70, 18)
	l.Title = "mac  —  select a project"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = pickerTitleStyle
	l.Styles.PaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	l.Styles.HelpStyle = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)

	return pickerModel{list: l}
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
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
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height - 2)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m pickerModel) View() string {
	return "\n" + m.list.View()
}
