package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/subbusainath/mac-cli/internal/credentials"
	"github.com/subbusainath/mac-cli/internal/db"
	"github.com/subbusainath/mac-cli/internal/scaffold"
)

// Run is the main TUI entry point.
// - If cwd matches a known project, reports it and exits.
// - Otherwise shows the project picker, then optionally the new-project wizard.
func Run(ctx context.Context, database *db.DB) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	existing, err := database.FindProjectByPath(ctx, cwd)
	if err != nil {
		return fmt.Errorf("check project: %w", err)
	}
	if existing != nil {
		fmt.Printf("%s %s %s\n",
			AccentStyle("◆  Active project:"),
			AccentStyle(existing.Name),
			DimStyle(existing.Path),
		)
		return nil
	}

	projects, err := database.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	// Show project picker.
	pickerProg := tea.NewProgram(newPickerModel(projects), tea.WithAltScreen())
	pickerResult, err := pickerProg.Run()
	if err != nil {
		return err
	}
	picker, ok := pickerResult.(pickerModel)
	if !ok || picker.quitting {
		return nil
	}
	if picker.choice != nil {
		fmt.Printf("%s %s\n",
			SuccessStyle("➜  Switched to project:"),
			SuccessStyle(picker.choice.Name),
		)
		return nil
	}

	// User chose "New Project" — run wizard.
	wizardProg := tea.NewProgram(newWizardModel(cwd), tea.WithAltScreen())
	wizardResult, err := wizardProg.Run()
	if err != nil {
		return err
	}
	wiz, ok := wizardResult.(wizardModel)
	if !ok || !wiz.done {
		return nil
	}

	if len(wiz.Answers.Keys) > 0 {
		keys := make(map[credentials.Provider]string, len(wiz.Answers.Keys))
		for p, k := range wiz.Answers.Keys {
			keys[credentials.Provider(p)] = k
		}
		if err := credentials.Save(keys); err != nil {
			return fmt.Errorf("save credentials: %w", err)
		}
	}

	if err := scaffold.New(ctx, database, wiz.Answers); err != nil {
		return fmt.Errorf("scaffold: %w", err)
	}

	// Styled success banner
	banner := lipgloss.NewStyle().
		Bold(true).
		Foreground(clrGreen).
		Background(clrOverlay).
		Padding(0, 1).
		Render("✓  Project ready")

	sep := lipgloss.NewStyle().
		Foreground(clrBorder).
		Render(strings.Repeat("─", max(0, 60-lipgloss.Width(banner))))

	fmt.Printf("\n  %s%s\n", banner, sep)
	fmt.Printf("  %s  %s\n",
		SuccessStyle("Name:"),
		AccentStyle(wiz.Answers.Name),
	)
	fmt.Printf("  %s  %s\n",
		DimStyle("Path:"),
		DimStyle(wiz.Answers.Path),
	)
	var stack []string
	for _, part := range []string{wiz.Answers.Backend, wiz.Answers.Frontend,
		wiz.Answers.Infra, wiz.Answers.Cloud, wiz.Answers.IAC} {
		if part != "" {
			stack = append(stack, part)
		}
	}
	fmt.Printf("  %s  %s\n", DimStyle("Stack:"), strings.Join(stack, " / "))
	fmt.Printf("  %s  planner %s/%s · coder %s/%s\n",
		DimStyle("Agents:"),
		wiz.Answers.Planner.Provider, wiz.Answers.Planner.Model,
		wiz.Answers.Coder.Provider, wiz.Answers.Coder.Model)
	fmt.Println()
	return nil
}
