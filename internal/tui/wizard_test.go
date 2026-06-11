package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/subbusainath/mac-cli/internal/credentials"
)

func cleanEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	for _, p := range credentials.All {
		t.Setenv(credentials.EnvVar(p), "")
	}
}

func pick(t *testing.T, l *list.Model, label string) {
	t.Helper()
	for i, it := range l.Items() {
		if it.(choiceItem).label == label {
			l.Select(i)
			return
		}
	}
	t.Fatalf("label %q not in list", label)
}

// drive answers name+path and the backend/frontend gates with "no".
func driveToInfra(t *testing.T) wizardModel {
	t.Helper()
	m := newWizardModel("/tmp")
	m.nameInput.SetValue("demo")
	mm, _ := m.advance() // name
	m = mm.(wizardModel)
	mm, _ = m.advance() // path (default cwd)
	m = mm.(wizardModel)
	pick(t, &m.wantBackend, "no")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	pick(t, &m.wantFrontend, "no")
	mm, _ = m.advance()
	return mm.(wizardModel)
}

func TestInfraNoSkipsCloudAndGoesToKeys(t *testing.T) {
	cleanEnv(t)
	m := driveToInfra(t)
	if m.step != stepWantInfra {
		t.Fatalf("step = %v, want stepWantInfra", m.step)
	}
	pick(t, &m.wantInfra, "no")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	if m.step != stepKeyGate {
		t.Fatalf("step = %v, want stepKeyGate (skip infra target, cloud, iac)", m.step)
	}
	if m.Answers.Infra != "" || m.Answers.Cloud != "" || m.Answers.IAC != "" {
		t.Fatalf("declined infra must clear: %+v", m.Answers)
	}
}

func TestCloudNoSkipsIaC(t *testing.T) {
	cleanEnv(t)
	m := driveToInfra(t)
	pick(t, &m.wantInfra, "yes")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	if m.step != stepInfraTarget {
		t.Fatalf("step = %v", m.step)
	}
	pick(t, &m.infraTarget, "containers")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	if m.step != stepWantCloud {
		t.Fatalf("step = %v", m.step)
	}
	pick(t, &m.wantCloud, "no")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	if m.step != stepKeyGate {
		t.Fatalf("step = %v, want stepKeyGate", m.step)
	}
	if m.Answers.Infra != "containers" || m.Answers.Cloud != "" {
		t.Fatalf("answers: %+v", m.Answers)
	}
}

func TestAllKeysDeclinedLeavesOnlyLocal(t *testing.T) {
	cleanEnv(t)
	m := driveToInfra(t)
	pick(t, &m.wantInfra, "no")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	// Decline all four providers.
	for i := 0; i < 4; i++ {
		if m.step != stepKeyGate {
			t.Fatalf("iteration %d: step = %v", i, m.step)
		}
		pick(t, &m.keyGate, "no")
		mm, _ = m.advance()
		m = mm.(wizardModel)
	}
	if m.step != stepPlanner {
		t.Fatalf("step = %v, want stepPlanner", m.step)
	}
	items := m.plannerPick.Items()
	if len(items) != 1 || items[0].(choiceItem).label != "local" {
		t.Fatalf("planner choices = %v, want only local", items)
	}
}

func TestDetectedKeySkipsGate(t *testing.T) {
	cleanEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-present")
	m := driveToInfra(t)
	pick(t, &m.wantInfra, "no")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	// First gate must be openai (anthropic auto-detected, openai asked first anyway);
	// decline remaining gates and confirm anthropic appears in planner list.
	for m.step == stepKeyGate {
		pick(t, &m.keyGate, "no")
		mm, _ = m.advance()
		m = mm.(wizardModel)
	}
	if m.step != stepPlanner {
		t.Fatalf("step = %v", m.step)
	}
	var labels []string
	for _, it := range m.plannerPick.Items() {
		labels = append(labels, it.(choiceItem).label)
	}
	if labels[0] != "anthropic" {
		t.Fatalf("planner default should be anthropic (strongest with key), got %v", labels)
	}
}

func TestFullKeyFlowThroughConfirm(t *testing.T) {
	cleanEnv(t)
	m := driveToInfra(t)
	pick(t, &m.wantInfra, "no")
	mm, _ := m.advance()
	m = mm.(wizardModel)
	// Say yes to openai, paste key, decline rest.
	pick(t, &m.keyGate, "yes")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	if m.step != stepKeyInput {
		t.Fatalf("step = %v", m.step)
	}
	m.keyInput.SetValue("sk-pasted")
	mm, _ = m.advance()
	m = mm.(wizardModel)
	for m.step == stepKeyGate {
		pick(t, &m.keyGate, "no")
		mm, _ = m.advance()
		m = mm.(wizardModel)
	}
	if m.Answers.Keys["openai"] != "sk-pasted" {
		t.Fatalf("keys = %v", m.Answers.Keys)
	}
	// planner: openai picked by default
	mm, _ = m.advance() // accept planner provider
	m = mm.(wizardModel)
	if m.step != stepPlannerModel {
		t.Fatalf("step = %v", m.step)
	}
	if m.plannerModel.Value() != "gpt-4o" {
		t.Fatalf("planner model default = %q", m.plannerModel.Value())
	}
	mm, _ = m.advance() // accept model
	m = mm.(wizardModel)
	mm, _ = m.advance() // accept coder provider (local default)
	m = mm.(wizardModel)
	mm, _ = m.advance() // accept coder model
	m = mm.(wizardModel)
	if m.step != stepConfirm {
		t.Fatalf("step = %v, want stepConfirm", m.step)
	}
	if m.Answers.Planner.Provider != "openai" || m.Answers.Planner.Model != "gpt-4o" {
		t.Fatalf("planner = %+v", m.Answers.Planner)
	}
	if m.Answers.Coder.Provider != "local" {
		t.Fatalf("coder = %+v", m.Answers.Coder)
	}
}
