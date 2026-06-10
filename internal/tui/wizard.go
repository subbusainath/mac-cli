package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subbusainath/mac-cli/internal/scaffold"
)

type wizardStep int

const (
	stepName wizardStep = iota
	stepPath
	stepBackend
	stepFrontend
	stepCloud
	stepIaC
	stepConfirm
)

var (
	wizHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	wizSubStyle    = lipgloss.NewStyle().Faint(true)
	wizErrStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	wizCheckStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
)

// choiceItem is a selectable list entry with a label and description.
type choiceItem struct{ label, desc string }

func (c choiceItem) FilterValue() string { return c.label }
func (c choiceItem) Title() string       { return c.label }
func (c choiceItem) Description() string { return c.desc }

var backendChoices = []list.Item{
	choiceItem{"fastapi", "Python + FastAPI  (uv package manager)"},
	choiceItem{"express", "Node.js + TypeScript + Express  (pnpm)"},
	choiceItem{"gin", "Go + Gin framework"},
	choiceItem{"axum", "Rust + Tokio / Axum  (cargo)"},
	choiceItem{"springboot", "Java + Spring Boot & Spring Cloud"},
}

var frontendChoices = []list.Item{
	choiceItem{"vanilla", "Vanilla  HTML / CSS / JS"},
	choiceItem{"react", "React  JS / TS"},
	choiceItem{"nextjs", "Next.js"},
	choiceItem{"svelte", "Svelte"},
}

var cloudChoices = []list.Item{
	choiceItem{"aws", "Amazon Web Services"},
	choiceItem{"azure", "Microsoft Azure"},
	choiceItem{"gcp", "Google Cloud Platform"},
}

var iacByCloud = map[string][]list.Item{
	"aws": {
		choiceItem{"cdk", "AWS CDK  (TypeScript)"},
		choiceItem{"terraform", "Terraform"},
		choiceItem{"sam", "AWS SAM"},
		choiceItem{"pulumi", "Pulumi"},
	},
	"azure": {
		choiceItem{"terraform", "Terraform"},
		choiceItem{"bicep", "Bicep"},
		choiceItem{"pulumi", "Pulumi"},
	},
	"gcp": {
		choiceItem{"terraform", "Terraform"},
		choiceItem{"pulumi", "Pulumi"},
		choiceItem{"deployment-manager", "Deployment Manager"},
	},
}

func newChoiceList(items []list.Item, title string) list.Model {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#7C3AED")).
		BorderLeftForeground(lipgloss.Color("#7C3AED"))
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#A78BFA"))

	l := list.New(items, d, 65, 14)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = wizHeaderStyle
	return l
}

type wizardModel struct {
	step      wizardStep
	nameInput textinput.Model
	pathInput textinput.Model
	backend   list.Model
	frontend  list.Model
	cloud     list.Model
	iac       list.Model
	Answers   scaffold.Answers
	done      bool
	err       string
}

func newWizardModel(cwd string) wizardModel {
	name := textinput.New()
	name.Placeholder = "my-awesome-project"
	name.CharLimit = 64
	name.Width = 40
	name.Focus()

	path := textinput.New()
	path.SetValue(cwd)
	path.CharLimit = 256
	path.Width = 60

	return wizardModel{
		step:      stepName,
		nameInput: name,
		pathInput: path,
		backend:   newChoiceList(backendChoices, "Backend stack"),
		frontend:  newChoiceList(frontendChoices, "Frontend stack"),
		cloud:     newChoiceList(cloudChoices, "Cloud provider"),
	}
}

func (m wizardModel) Init() tea.Cmd { return textinput.Blink }

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m.advance()
		case "esc":
			return m.back()
		}
	case tea.WindowSizeMsg:
		w := msg.Width - 4
		m.backend.SetWidth(w)
		m.frontend.SetWidth(w)
		m.cloud.SetWidth(w)
		m.iac.SetWidth(w)
	}

	var cmd tea.Cmd
	switch m.step {
	case stepName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case stepPath:
		m.pathInput, cmd = m.pathInput.Update(msg)
	case stepBackend:
		m.backend, cmd = m.backend.Update(msg)
	case stepFrontend:
		m.frontend, cmd = m.frontend.Update(msg)
	case stepCloud:
		m.cloud, cmd = m.cloud.Update(msg)
	case stepIaC:
		m.iac, cmd = m.iac.Update(msg)
	}
	return m, cmd
}

func (m wizardModel) advance() (tea.Model, tea.Cmd) {
	m.err = ""
	switch m.step {
	case stepName:
		val := strings.TrimSpace(m.nameInput.Value())
		if val == "" {
			m.err = "project name is required"
			return m, nil
		}
		m.Answers.Name = val
		m.nameInput.Blur()
		m.pathInput.Focus()
		m.step = stepPath

	case stepPath:
		val := strings.TrimSpace(m.pathInput.Value())
		if val == "" {
			val, _ = os.Getwd()
		}
		m.Answers.Path = val
		m.pathInput.Blur()
		m.step = stepBackend

	case stepBackend:
		item, ok := m.backend.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Backend = item.label
		m.step = stepFrontend

	case stepFrontend:
		item, ok := m.frontend.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Frontend = item.label
		m.step = stepCloud

	case stepCloud:
		item, ok := m.cloud.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Cloud = item.label
		m.iac = newChoiceList(iacByCloud[item.label], "Infrastructure as Code")
		m.step = stepIaC

	case stepIaC:
		item, ok := m.iac.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.IAC = item.label
		m.step = stepConfirm

	case stepConfirm:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m wizardModel) back() (tea.Model, tea.Cmd) {
	if m.step > stepName {
		m.step--
		m.err = ""
	}
	return m, nil
}

func (m wizardModel) View() string {
	var b strings.Builder
	b.WriteString(wizHeaderStyle.Render("mac  —  new project") + "\n\n")

	if m.err != "" {
		b.WriteString(wizErrStyle.Render("⚠  "+m.err) + "\n\n")
	}

	hint := wizSubStyle.Render("enter to continue  •  esc to go back  •  ctrl+c to quit")

	switch m.step {
	case stepName:
		b.WriteString(wizSubStyle.Render("Step 1 / 6  —  Project name") + "\n")
		b.WriteString(m.nameInput.View() + "\n\n")
		b.WriteString(wizSubStyle.Render("enter to continue  •  ctrl+c to quit"))

	case stepPath:
		b.WriteString(wizSubStyle.Render("Step 2 / 6  —  Project path") + "\n")
		b.WriteString(m.pathInput.View() + "\n\n")
		b.WriteString(hint)

	case stepBackend:
		b.WriteString(wizSubStyle.Render("Step 3 / 6") + "\n")
		b.WriteString(m.backend.View())

	case stepFrontend:
		b.WriteString(wizSubStyle.Render("Step 4 / 6") + "\n")
		b.WriteString(m.frontend.View())

	case stepCloud:
		b.WriteString(wizSubStyle.Render("Step 5 / 6") + "\n")
		b.WriteString(m.cloud.View())

	case stepIaC:
		b.WriteString(wizSubStyle.Render("Step 6 / 6") + "\n")
		b.WriteString(m.iac.View())

	case stepConfirm:
		b.WriteString(wizHeaderStyle.Render("Confirm  —  press enter to scaffold") + "\n\n")
		rows := []struct{ k, v string }{
			{"Name", m.Answers.Name},
			{"Path", m.Answers.Path},
			{"Backend", m.Answers.Backend},
			{"Frontend", m.Answers.Frontend},
			{"Cloud", m.Answers.Cloud},
			{"IaC", m.Answers.IAC},
		}
		for _, r := range rows {
			b.WriteString(fmt.Sprintf("  %s  %-10s %s\n",
				wizCheckStyle.Render("✓"),
				r.k+":",
				r.v,
			))
		}
		b.WriteString("\n" + hint)
	}

	return b.String()
}
