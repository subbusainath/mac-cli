package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const macLogo = `███╗   ███╗ █████╗  ██████╗
████╗ ████║██╔══██╗██╔════╝
██╔████╔██║███████║██║
██║╚██╔╝██║██╔══██║██║
██║ ╚═╝ ██║██║  ██║╚██████╗
╚═╝     ╚═╝╚═╝  ╚═╝ ╚═════╝`

const tagline = "My Agentic CLI"

// lerpHex linearly interpolates two #RRGGBB colors.
func lerpHex(a, b string, t float64) string {
	parse := func(s string) (int64, int64, int64) {
		var r, g, bl int64
		fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &bl)
		return r, g, bl
	}
	ar, ag, ab := parse(a)
	br, bg, bb := parse(b)
	mix := func(x, y int64) int64 { return x + int64(t*float64(y-x)) }
	return fmt.Sprintf("#%02X%02X%02X", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

// renderBannerFrame renders the banner at animation progress t in [0,1].
// Gradient sweeps left→right with t; tagline typewriter-expands with t.
func renderBannerFrame(t float64) string {
	lines := strings.Split(macLogo, "\n")
	width := 0
	for _, l := range lines {
		if n := len([]rune(l)); n > width {
			width = n
		}
	}

	var b strings.Builder
	for _, line := range lines {
		for i, r := range []rune(line) {
			pos := float64(i) / float64(width)
			// cells past the sweep front stay muted
			clr := string(clrBorder)
			if pos <= t {
				clr = lerpHex(string(clrPurple), string(clrBlue), pos)
			}
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(clr)).Render(string(r)))
		}
		b.WriteString("\n")
	}

	shown := int(t * float64(len(tagline)))
	if shown > len(tagline) {
		shown = len(tagline)
	}
	spaced := strings.Join(strings.Split(tagline[:shown], ""), " ")
	b.WriteString("        " + AccentStyle(spaced) + "\n")
	return b.String()
}

type bannerModel struct {
	t       float64
	done    bool
	skipped bool
}

type bannerTick struct{}

func tickBanner() tea.Cmd {
	return tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg { return bannerTick{} })
}

func (m bannerModel) Init() tea.Cmd { return tickBanner() }

func (m bannerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		m.skipped = true
		return m, tea.Quit
	case bannerTick:
		m.t += 0.05 // ~20 frames * 40ms = 800ms total
		if m.t >= 1.0 {
			m.t = 1.0
			m.done = true
			return m, tea.Quit
		}
		return m, tickBanner()
	}
	return m, nil
}

func (m bannerModel) View() string { return "\n" + renderBannerFrame(m.t) }

// ShowBanner plays the animated banner. It is skipped when stdout is not a
// TTY, when MAC_NO_BANNER is set, or when noBanner is true. Always ≤ 1s;
// any key skips.
func ShowBanner(noBanner bool) {
	if noBanner || os.Getenv("MAC_NO_BANNER") != "" ||
		!term.IsTerminal(int(os.Stdout.Fd())) {
		return
	}
	p := tea.NewProgram(bannerModel{})
	if _, err := p.Run(); err != nil {
		return // banner is cosmetic; never fail the CLI over it
	}
	fmt.Print(renderBannerFrame(1.0)) // leave the final frame on screen
}

// CompactBrand is the one-line header used by subcommands.
func CompactBrand() string {
	return AccentStyle("◆ MAC") + DimStyle(" · "+tagline)
}
