package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	fmt.Printf("  %s  %s / %s / %s / %s\n",
		DimStyle("Stack:"),
		wiz.Answers.Backend,
		wiz.Answers.Frontend,
		wiz.Answers.Cloud,
		wiz.Answers.IAC,
	)
	fmt.Println()
	return nil
}
