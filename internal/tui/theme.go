package tui

import "github.com/charmbracelet/lipgloss"

// ── Tokyo Night inspired palette ─────────────────────────────────────────

var (
	clrPurple  = lipgloss.Color("#7C3AED")
	clrBlue    = lipgloss.Color("#3B82F6")
	clrGreen   = lipgloss.Color("#10B981")
	clrAmber   = lipgloss.Color("#F59E0B")
	clrRed     = lipgloss.Color("#EF4444")
	clrCyan    = lipgloss.Color("#06B6D4")
	clrText    = lipgloss.Color("#E2E8F0")
	clrMuted   = lipgloss.Color("#6B7280")
	clrBorder  = lipgloss.Color("#334155")
	clrSurface = lipgloss.Color("#1E293B")
	clrOverlay = lipgloss.Color("#0F172A")
)

// ── Shared component styles ───────────────────────────────────────────────

var (
	// Banner — big app title header
	BannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrPurple).
			Background(clrOverlay).
			Padding(0, 2).
			Render

	// Subtitle — smaller muted heading
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(clrMuted).
			Render

	// Accent label
	AccentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrPurple).
			Render

	// Success label
	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrGreen).
			Render

	// Error label
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrRed).
			Render

	// Dimmed / secondary text
	DimStyle = lipgloss.NewStyle().
			Foreground(clrMuted).
			Render

	// Faint hint text
	HintStyle = lipgloss.NewStyle().
			Foreground(clrMuted).
			Render

	// Selected item
	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrPurple).
			Render

	// Checkmark ✓
	CheckStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrGreen).
			Render

	// Step number
	StepNumStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrBlue).
			Render

	// Key name in confirm screen
	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(clrCyan).
			Render
)

// ── Step progress dots ────────────────────────────────────────────────────

var (
	dotDone    = lipgloss.NewStyle().Foreground(clrPurple).Render("●")
	dotCurrent = lipgloss.NewStyle().Foreground(clrBlue).Bold(true).Render("●")
	dotTodo    = lipgloss.NewStyle().Foreground(clrBorder).Render("○")
)

func stepDots(current, total int) string {
	var s string
	for i := range total {
		if i > 0 {
			s += " "
		}
		switch {
		case i < current-1:
			s += dotDone
		case i == current-1:
			s += dotCurrent
		default:
			s += dotTodo
		}
	}
	return s
}
