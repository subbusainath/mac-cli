package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subbusainath/mac-cli/internal/credentials"
	"github.com/subbusainath/mac-cli/internal/scaffold"
)

type wizardStep int

const (
	stepName wizardStep = iota
	stepPath
	stepWantBackend
	stepBackend
	stepWantFrontend
	stepFrontend
	stepWantInfra
	stepInfraTarget
	stepWantCloud
	stepCloud
	stepIaC
	stepKeyGate
	stepKeyInput
	stepPlanner
	stepPlannerModel
	stepCoder
	stepCoderModel
	stepConfirm
)

var (
	stepNames = map[wizardStep]string{
		stepName:         "Project Name",
		stepPath:         "Project Path",
		stepWantBackend:  "Include Backend?",
		stepBackend:      "Backend Stack",
		stepWantFrontend: "Include Frontend?",
		stepFrontend:     "Frontend Stack",
		stepWantInfra:    "Infrastructure?",
		stepInfraTarget:  "Deployment Target",
		stepWantCloud:    "Cloud Provider?",
		stepCloud:        "Cloud Provider",
		stepIaC:          "Infrastructure as Code",
		stepKeyGate:      "API Keys",
		stepKeyInput:     "API Keys",
		stepPlanner:      "Agents",
		stepPlannerModel: "Agents",
		stepCoder:        "Agents",
		stepCoderModel:   "Agents",
		stepConfirm:      "Confirm",
	}
	stepLabels = map[wizardStep]string{
		stepName:         "What should we call your project?",
		stepPath:         "Where should we create it?",
		stepWantBackend:  "Do you want a backend API?",
		stepBackend:      "Choose your backend framework",
		stepWantFrontend: "Do you want a frontend UI?",
		stepFrontend:     "Choose your frontend framework",
		stepWantInfra:    "Include infrastructure (containers / K8s / cloud)?",
		stepInfraTarget:  "Choose your deployment target",
		stepWantCloud:    "Deploy to a cloud provider?",
		stepCloud:        "Which cloud provider?",
		stepIaC:          "Pick your Infrastructure as Code tool",
	}
	progressOrder = []wizardStep{
		stepName, stepPath, stepWantBackend, stepWantFrontend,
		stepWantInfra, stepWantCloud, stepKeyGate, stepPlanner,
	}
)

// progressSlot maps detail steps to their gate's slot for the progress bar.
func progressSlot(s wizardStep) wizardStep {
	switch s {
	case stepBackend:
		return stepWantBackend
	case stepFrontend:
		return stepWantFrontend
	case stepInfraTarget:
		return stepWantInfra
	case stepCloud, stepIaC:
		return stepWantCloud
	case stepKeyInput:
		return stepKeyGate
	case stepPlannerModel, stepCoder, stepCoderModel:
		return stepPlanner
	default:
		return s
	}
}

// ── Choice item for lists ─────────────────────────────────────────────────

type choiceItem struct{ label, desc string }

func (c choiceItem) FilterValue() string { return c.label }
func (c choiceItem) Title() string       { return c.label }
func (c choiceItem) Description() string { return c.desc }

// ── Yes/No choice ─────────────────────────────────────────────────────────

var yesNoItems = []list.Item{
	choiceItem{"yes", ""},
	choiceItem{"no", ""},
}

var backendChoices = []list.Item{
	choiceItem{"fastapi", "Python + FastAPI (uv)"},
	choiceItem{"express", "Node.js + TypeScript + Express (pnpm)"},
	choiceItem{"gin", "Go + Gin framework"},
	choiceItem{"axum", "Rust + Tokio / Axum (cargo)"},
	choiceItem{"springboot", "Java + Spring Boot & Spring Cloud"},
}

var frontendChoices = []list.Item{
	choiceItem{"vanilla", "Vanilla HTML / CSS / JS"},
	choiceItem{"react", "React JS / TS"},
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
		choiceItem{"cdk", "AWS CDK (TypeScript)"},
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

var infraTargetChoices = []list.Item{
	choiceItem{"local", "Run on this machine only — no container files"},
	choiceItem{"containers", "Dockerfiles + docker-compose"},
	choiceItem{"k8s", "Containers + Kubernetes manifests"},
}

var defaultModels = map[string]string{
	"openai":     "gpt-4o",
	"anthropic":  "claude-sonnet-4-6",
	"deepseek":   "deepseek-chat",
	"openrouter": "openrouter/auto",
	"local":      "qwen2.5-coder:14b",
}

var providerTitles = map[credentials.Provider]string{
	credentials.OpenAI:     "OpenAI",
	credentials.Anthropic:  "Claude (Anthropic)",
	credentials.DeepSeek:   "DeepSeek",
	credentials.OpenRouter: "OpenRouter",
}

// plannerOrder: strongest first; coderOrder: cheapest first.
var plannerOrder = []string{"anthropic", "openai", "openrouter", "deepseek", "local"}
var coderOrder = []string{"local", "deepseek", "openrouter", "openai", "anthropic"}

// ── Styled choice list ────────────────────────────────────────────────────

func newChoiceList(items []list.Item, title string) list.Model {
	d := list.NewDefaultDelegate()
	d.ShowDescription = true
	d.SetSpacing(0)

	d.Styles.NormalTitle = lipgloss.NewStyle().
		PaddingLeft(4).
		Foreground(clrText)
	d.Styles.NormalDesc = lipgloss.NewStyle().
		PaddingLeft(6).
		Foreground(clrMuted)
	d.Styles.SelectedTitle = lipgloss.NewStyle().
		PaddingLeft(2).
		Bold(true).
		Foreground(clrPurple)
	d.Styles.SelectedDesc = lipgloss.NewStyle().
		PaddingLeft(4).
		Foreground(clrBlue)

	l := list.New(items, d, 65, 12)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	sty := list.DefaultStyles()
	sty.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(clrPurple).
		PaddingLeft(2)
	l.Styles = sty

	return l
}

// ── Styled text input ─────────────────────────────────────────────────────

func newStyledInput() textinput.Model {
	input := textinput.New()
	input.CharLimit = 64
	input.Width = 50
	input.TextStyle = lipgloss.NewStyle().Foreground(clrText)
	return input
}

// ── Model ─────────────────────────────────────────────────────────────────

type wizardModel struct {
	step         wizardStep
	nameInput    textinput.Model
	pathInput    textinput.Model
	wantBackend  list.Model
	backend      list.Model
	wantFrontend list.Model
	frontend     list.Model
	wantInfra    list.Model
	infraTarget  list.Model
	wantCloud    list.Model
	cloud        list.Model
	iac          list.Model
	keyGate      list.Model
	keyInput     textinput.Model
	keyIdx       int
	keyStatus    map[credentials.Provider]string // "env" | "file" | "wizard"
	plannerPick  list.Model
	plannerModel textinput.Model
	coderPick    list.Model
	coderModel   textinput.Model
	Answers      scaffold.Answers
	done         bool
	err          string
	width        int
	height       int
}

func newWizardModel(cwd string) wizardModel {
	name := newStyledInput()
	name.Placeholder = "my-awesome-project"
	name.Focus()

	path := newStyledInput()
	path.SetValue(cwd)
	path.CharLimit = 256

	key := newStyledInput()
	key.EchoMode = textinput.EchoPassword
	key.CharLimit = 256

	return wizardModel{
		step:         stepName,
		nameInput:    name,
		pathInput:    path,
		wantBackend:  newChoiceList(yesNoItems, "Include backend?"),
		backend:      newChoiceList(backendChoices, "Select backend"),
		wantFrontend: newChoiceList(yesNoItems, "Include frontend?"),
		frontend:     newChoiceList(frontendChoices, "Select frontend"),
		wantInfra:    newChoiceList(yesNoItems, "Include infrastructure?"),
		infraTarget:  newChoiceList(infraTargetChoices, "Deployment target"),
		wantCloud:    newChoiceList(yesNoItems, "Deploy to cloud?"),
		keyInput:     key,
		keyStatus:    map[credentials.Provider]string{},
		plannerModel: newStyledInput(),
		coderModel:   newStyledInput(),
		width:        80,
		height:       24,
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
		m.width = msg.Width
		m.height = msg.Height
		w := msg.Width - 6
		m.nameInput.Width = max(30, w-10)
		m.pathInput.Width = max(30, w-10)
		m.wantBackend.SetWidth(w)
		m.backend.SetWidth(w)
		m.wantFrontend.SetWidth(w)
		m.frontend.SetWidth(w)
		m.wantInfra.SetWidth(w)
		m.infraTarget.SetWidth(w)
		m.wantCloud.SetWidth(w)
		m.cloud.SetWidth(w)
		if m.step >= stepIaC {
			m.iac.SetWidth(w)
		}
	}

	var cmd tea.Cmd
	switch m.step {
	case stepName:
		m.nameInput, cmd = m.nameInput.Update(msg)
	case stepPath:
		m.pathInput, cmd = m.pathInput.Update(msg)
	case stepWantBackend:
		m.wantBackend, cmd = m.wantBackend.Update(msg)
	case stepBackend:
		m.backend, cmd = m.backend.Update(msg)
	case stepWantFrontend:
		m.wantFrontend, cmd = m.wantFrontend.Update(msg)
	case stepFrontend:
		m.frontend, cmd = m.frontend.Update(msg)
	case stepWantInfra:
		m.wantInfra, cmd = m.wantInfra.Update(msg)
	case stepInfraTarget:
		m.infraTarget, cmd = m.infraTarget.Update(msg)
	case stepWantCloud:
		m.wantCloud, cmd = m.wantCloud.Update(msg)
	case stepCloud:
		m.cloud, cmd = m.cloud.Update(msg)
	case stepIaC:
		m.iac, cmd = m.iac.Update(msg)
	case stepKeyGate:
		m.keyGate, cmd = m.keyGate.Update(msg)
	case stepKeyInput:
		m.keyInput, cmd = m.keyInput.Update(msg)
	case stepPlanner:
		m.plannerPick, cmd = m.plannerPick.Update(msg)
	case stepPlannerModel:
		m.plannerModel, cmd = m.plannerModel.Update(msg)
	case stepCoder:
		m.coderPick, cmd = m.coderPick.Update(msg)
	case stepCoderModel:
		m.coderModel, cmd = m.coderModel.Update(msg)
	}
	return m, cmd
}

// ── Key phase helpers ─────────────────────────────────────────────────────

// enterKeyPhase advances keyIdx past providers whose key is already known
// and returns the next step: another gate, or the planner pick.
func (m *wizardModel) enterKeyPhase() wizardStep {
	for m.keyIdx < len(credentials.All) {
		p := credentials.All[m.keyIdx]
		if _, src, ok := credentials.Lookup(p); ok {
			m.keyStatus[p] = src
			m.keyIdx++
			continue
		}
		title := fmt.Sprintf("Have a %s API key?", providerTitles[p])
		m.keyGate = newChoiceList(yesNoItems, title)
		return stepKeyGate
	}
	m.buildAgentLists()
	return stepPlanner
}

// availableProviders returns providers with keys, ordered by pref, plus local.
func (m *wizardModel) availableProviders(order []string) []list.Item {
	var items []list.Item
	for _, prov := range order {
		if prov == "local" {
			items = append(items, choiceItem{"local",
				"Ollama at localhost:11434 — " + defaultModels["local"]})
			continue
		}
		if m.keyStatus[credentials.Provider(prov)] != "" {
			items = append(items, choiceItem{prov, "default: " + defaultModels[prov]})
		}
	}
	return items
}

func (m *wizardModel) buildAgentLists() {
	m.plannerPick = newChoiceList(m.availableProviders(plannerOrder), "Planner model provider")
	m.coderPick = newChoiceList(m.availableProviders(coderOrder), "Coder model provider")
}

// ── Advance / Back ────────────────────────────────────────────────────────

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
		m.step = stepWantBackend

	case stepWantBackend:
		item, ok := m.wantBackend.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		if item.label == "yes" {
			m.step = stepBackend
		} else {
			m.Answers.Backend = ""
			m.step = stepWantFrontend
		}

	case stepBackend:
		item, ok := m.backend.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Backend = item.label
		m.step = stepWantFrontend

	case stepWantFrontend:
		item, ok := m.wantFrontend.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		if item.label == "yes" {
			m.step = stepFrontend
		} else {
			m.Answers.Frontend = ""
			m.step = stepWantInfra
		}

	case stepFrontend:
		item, ok := m.frontend.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Frontend = item.label
		m.step = stepWantInfra

	case stepWantInfra:
		item, ok := m.wantInfra.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		if item.label == "yes" {
			m.step = stepInfraTarget
		} else {
			m.Answers.Infra, m.Answers.Cloud, m.Answers.IAC = "", "", ""
			m.step = m.enterKeyPhase()
		}

	case stepInfraTarget:
		item, ok := m.infraTarget.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Infra = item.label
		m.step = stepWantCloud

	case stepWantCloud:
		item, ok := m.wantCloud.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		if item.label == "yes" {
			m.step = stepCloud
		} else {
			m.Answers.Cloud, m.Answers.IAC = "", ""
			m.step = m.enterKeyPhase()
		}

	case stepCloud:
		item, ok := m.cloud.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Cloud = item.label
		m.iac = newChoiceList(iacByCloud[item.label], "Select IaC tool")
		m.step = stepIaC

	case stepIaC:
		item, ok := m.iac.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.IAC = item.label
		m.step = m.enterKeyPhase()

	case stepKeyGate:
		item, ok := m.keyGate.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		p := credentials.All[m.keyIdx]
		if item.label == "yes" {
			m.keyInput.SetValue("")
			m.keyInput.Placeholder = credentials.EnvVar(p) + " value"
			m.keyInput.Focus()
			m.step = stepKeyInput
		} else {
			m.keyIdx++
			m.step = m.enterKeyPhase()
		}

	case stepKeyInput:
		p := credentials.All[m.keyIdx]
		if val := strings.TrimSpace(m.keyInput.Value()); val != "" {
			if m.Answers.Keys == nil {
				m.Answers.Keys = map[string]string{}
			}
			m.Answers.Keys[string(p)] = val
			m.keyStatus[p] = "wizard"
		}
		m.keyInput.Blur()
		m.keyIdx++
		m.step = m.enterKeyPhase()

	case stepPlanner:
		item, ok := m.plannerPick.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Planner.Provider = item.label
		m.plannerModel.SetValue(defaultModels[item.label])
		m.plannerModel.Focus()
		m.step = stepPlannerModel

	case stepPlannerModel:
		m.Answers.Planner.Model = strings.TrimSpace(m.plannerModel.Value())
		if m.Answers.Planner.Model == "" {
			m.Answers.Planner.Model = defaultModels[m.Answers.Planner.Provider]
		}
		m.plannerModel.Blur()
		m.step = stepCoder

	case stepCoder:
		item, ok := m.coderPick.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		m.Answers.Coder.Provider = item.label
		m.coderModel.SetValue(defaultModels[item.label])
		m.coderModel.Focus()
		m.step = stepCoderModel

	case stepCoderModel:
		m.Answers.Coder.Model = strings.TrimSpace(m.coderModel.Value())
		if m.Answers.Coder.Model == "" {
			m.Answers.Coder.Model = defaultModels[m.Answers.Coder.Provider]
		}
		m.coderModel.Blur()
		m.step = stepConfirm

	case stepConfirm:
		m.done = true
		return m, tea.Quit
	}
	return m, nil
}

func (m wizardModel) back() (tea.Model, tea.Cmd) {
	if m.step <= stepName {
		return m, nil
	}
	m.err = ""

	switch m.step {
	case stepBackend:
		m.step = stepWantBackend
	case stepFrontend:
		m.step = stepWantFrontend
	case stepInfraTarget:
		m.step = stepWantInfra
	case stepCloud:
		m.step = stepWantCloud
	case stepKeyInput:
		m.step = stepKeyGate
	case stepKeyGate, stepPlanner:
		// Reset key phase state when backing out through it.
		m.keyIdx = 0
		for p, src := range m.keyStatus {
			if src != "wizard" {
				delete(m.keyStatus, p)
			}
		}
		m.step = stepWantInfra
	case stepPlannerModel:
		m.step = stepPlanner
	case stepCoder:
		m.step = stepPlannerModel
	case stepCoderModel:
		m.step = stepCoder
	default:
		m.step--
	}
	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────

func (m wizardModel) View() string {
	var b strings.Builder

	// ── Header ──────────────────────────────────────────────────────────
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(clrPurple).
		Render("◆  mac  ·  new project")
	b.WriteString(title)
	b.WriteString("\n")

	// ── Step progress bar ───────────────────────────────────────────────
	b.WriteString(renderProgress(m))
	b.WriteString("\n")

	// ── Error ───────────────────────────────────────────────────────────
	if m.err != "" {
		errBox := lipgloss.NewStyle().
			Foreground(clrRed).
			Bold(true).
			Padding(0, 1).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderTop(true).BorderBottom(true).BorderLeft(true).BorderRight(true).
			BorderForeground(clrRed).
			Render("⚠  " + m.err)
		b.WriteString(errBox)
		b.WriteString("\n\n")
	}

	// ── Step body ───────────────────────────────────────────────────────
	switch m.step {
	case stepName:
		b.WriteString(renderTextStep("Name your project", m.nameInput.View()))
	case stepPath:
		b.WriteString(renderTextStep("Set the project path", m.pathInput.View()))
	case stepWantBackend:
		b.WriteString(renderYesNoStep("Include a backend API server?", m.wantBackend))
	case stepBackend:
		b.WriteString(renderListStep(m.backend, "Backend Stack"))
	case stepWantFrontend:
		b.WriteString(renderYesNoStep("Include a frontend UI?", m.wantFrontend))
	case stepFrontend:
		b.WriteString(renderListStep(m.frontend, "Frontend Stack"))
	case stepWantInfra:
		b.WriteString(renderYesNoStep("Include infrastructure (containers / K8s / cloud)?", m.wantInfra))
	case stepInfraTarget:
		b.WriteString(renderListStep(m.infraTarget, "Deployment Target"))
	case stepWantCloud:
		b.WriteString(renderYesNoStep("Deploy to a cloud provider?", m.wantCloud))
	case stepCloud:
		b.WriteString(renderListStep(m.cloud, "Cloud Provider"))
	case stepIaC:
		b.WriteString(renderListStep(m.iac, "Infrastructure as Code"))
	case stepKeyGate:
		p := credentials.All[m.keyIdx]
		b.WriteString(renderKeyStatus(m.keyStatus))
		b.WriteString(renderYesNoStep(fmt.Sprintf("Have a %s API key?", providerTitles[p]), m.keyGate))
	case stepKeyInput:
		p := credentials.All[m.keyIdx]
		b.WriteString(renderTextStep(
			fmt.Sprintf("Paste your %s key (input hidden)", providerTitles[p]),
			m.keyInput.View()))
	case stepPlanner:
		b.WriteString(renderListStep(m.plannerPick, "Planner Provider"))
	case stepPlannerModel:
		b.WriteString(renderTextStep("Planner model (edit or press enter)", m.plannerModel.View()))
	case stepCoder:
		b.WriteString(renderListStep(m.coderPick, "Coder Provider"))
	case stepCoderModel:
		b.WriteString(renderTextStep("Coder model (edit or press enter)", m.coderModel.View()))
	case stepConfirm:
		b.WriteString(renderConfirm(m.Answers))
	}

	// ── Footer help ─────────────────────────────────────────────────────
	if m.step != stepConfirm {
		b.WriteString("\n" + m.renderHelp())
	}

	return b.String()
}

// ── Sub-renderers ─────────────────────────────────────────────────────────

func renderProgress(m wizardModel) string {
	ordered := progressOrder
	currentIdx := -1
	slot := progressSlot(m.step)
	for i, s := range ordered {
		if s == slot {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		currentIdx = len(ordered) // confirm step
	}

	var dots strings.Builder
	for i := range ordered {
		if i > 0 {
			dots.WriteString("  ")
		}
		switch {
		case i < currentIdx:
			dots.WriteString(dotDone)
		case i == currentIdx:
			dots.WriteString(dotCurrent)
		default:
			dots.WriteString(dotTodo)
		}
	}

	stepNum := currentIdx + 1
	total := len(ordered)
	label, ok := stepNames[m.step]
	if !ok {
		label = "Confirm"
	}

	progressLine := lipgloss.JoinHorizontal(lipgloss.Center,
		lipgloss.NewStyle().Foreground(clrMuted).Render(fmt.Sprintf("Step %d/%d", stepNum, total)),
		lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1).Foreground(clrBorder).Render("·"),
		AccentStyle(label),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().PaddingLeft(2).Render(dots.String()),
		lipgloss.NewStyle().PaddingLeft(2).Render(progressLine),
	)
}

func renderTextStep(label, input string) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(clrMuted).
		PaddingLeft(2).
		Render(label))
	b.WriteString("\n")

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(true).BorderBottom(true).BorderLeft(true).BorderRight(true).
		BorderForeground(clrBorder).
		Padding(0, 1).
		Width(54).
		Render(input)

	b.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(box))
	return b.String()
}

func renderYesNoStep(label string, l list.Model) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(clrMuted).
		PaddingLeft(2).
		Render(label))
	b.WriteString("\n\n")

	d := list.NewDefaultDelegate()
	d.SetSpacing(0)
	d.Styles.NormalTitle = lipgloss.NewStyle().
		PaddingLeft(6).
		Foreground(clrText)
	d.Styles.SelectedTitle = lipgloss.NewStyle().
		PaddingLeft(4).
		Bold(true).
		Foreground(clrPurple)

	yn := list.New(yesNoItems, d, 20, 2)
	yn.SetShowTitle(false)
	yn.SetShowStatusBar(false)
	yn.SetShowPagination(false)
	yn.SetShowHelp(false)
	yn.SetFilteringEnabled(false)
	yn.DisableQuitKeybindings()
	yn.SetHeight(2)

	yn.Select(l.Index())

	b.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(yn.View()))
	return b.String()
}

func renderListStep(l list.Model, title string) string {
	l.Title = title
	return l.View()
}

// renderKeyStatus shows ✓ badges for providers already resolved.
func renderKeyStatus(status map[credentials.Provider]string) string {
	var parts []string
	for _, p := range credentials.All {
		if src := status[p]; src != "" {
			parts = append(parts, CheckStyle("✓ ")+string(p)+DimStyle(" ("+src+")"))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(parts, "   ")) + "\n\n"
}

func renderConfirm(a scaffold.Answers) string {
	var b strings.Builder

	readyStyle := lipgloss.NewStyle().Bold(true).Foreground(clrGreen).PaddingLeft(2)
	b.WriteString(readyStyle.Render("✓  Ready to scaffold"))
	b.WriteString("\n")

	rows := []struct{ k, v string }{
		{"Project", a.Name},
		{"Path", a.Path},
		{"Backend", iface(a.Backend)},
		{"Frontend", iface(a.Frontend)},
		{"Infra", iface(a.Infra)},
		{"Cloud", iface(a.Cloud)},
		{"IaC", iface(a.IAC)},
		{"Planner", a.Planner.Provider + " / " + a.Planner.Model},
		{"Coder", a.Coder.Provider + " / " + a.Coder.Model},
	}

	maxW := 0
	for _, r := range rows {
		if len(r.k) > maxW {
			maxW = len(r.k)
		}
	}

	var card strings.Builder
	card.WriteString("  ")
	if a.Backend == "" && a.Frontend == "" {
		card.WriteString(CheckStyle("!"))
	} else {
		card.WriteString(CheckStyle("✓"))
	}
	card.WriteString("  ")
	card.WriteString(KeyStyle(fmt.Sprintf("%-*s", maxW, rows[0].k)))
	card.WriteString("  ")
	card.WriteString(AccentStyle(rows[0].v))
	card.WriteString("\n")

	for _, r := range rows[1:] {
		card.WriteString("     ")
		card.WriteString(DimStyle("·"))
		card.WriteString("  ")
		card.WriteString(KeyStyle(fmt.Sprintf("%-*s", maxW, r.k)))
		card.WriteString("  ")
		card.WriteString(r.v)
		card.WriteString("\n")
	}

	box := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(true).BorderBottom(true).BorderLeft(true).BorderRight(true).
		BorderForeground(clrGreen).
		Padding(1, 2).
		Width(56).
		Render(card.String())

	b.WriteString(lipgloss.NewStyle().PaddingLeft(2).Render(box))
	b.WriteString("\n\n")

	b.WriteString(HintStyle("  enter  ") + SuccessStyle("✓ scaffold") + HintStyle("  ·  esc  ← back  ·  ctrl+c  ✗ quit"))

	return b.String()
}

func iface(v string) string {
	if v == "" {
		return DimStyle("(none)")
	}
	return v
}

func (m wizardModel) renderHelp() string {
	parts := []string{
		HintStyle("enter  ") + SuccessStyle("✓  next"),
		HintStyle("esc  ") + AccentStyle("←  back"),
		HintStyle("ctrl+c  ") + ErrorStyle("✗  quit"),
	}
	return "  " + strings.Join(parts, HintStyle("  ·  "))
}
