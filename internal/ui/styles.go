package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Brand Colors
	BrandGreen     = lipgloss.Color("#00a859")
	BrandGreenDark = lipgloss.Color("#004d28")
	BrandEmerald   = lipgloss.Color("#10b981")
	BgDark         = lipgloss.Color("#0f172a")
	BgCard         = lipgloss.Color("#1e293b")
	BorderColor    = lipgloss.Color("#334155")
	TextPrimary    = lipgloss.Color("#f8fafc")
	TextSecondary  = lipgloss.Color("#94a3b8")
	TextMuted      = lipgloss.Color("#64748b")
	AccentYellow   = lipgloss.Color("#f59e0b")
	AccentRed      = lipgloss.Color("#ef4444")
	AccentBlue     = lipgloss.Color("#38bdf8")
	BrandCyan      = lipgloss.Color("#38bdf8")

	// App Layout Styles
	AppStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// App Window Container (Harlequin / Btop style unified framed box)
	AppContainerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BrandEmerald).
			Background(lipgloss.Color("#111827")).
			Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextPrimary).
			Background(BrandGreenDark).
			Padding(0, 1).
			MarginBottom(1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrandEmerald)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(TextSecondary)

	// Tabs
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextPrimary).
			Background(BrandGreen).
			Padding(0, 2)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(TextSecondary).
				Background(BgCard).
				Padding(0, 2)

	TabGapStyle = lipgloss.NewStyle().
			Foreground(BorderColor)

	// Cards & Containers
	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Background(BgCard).
			Padding(1, 2)

	ActiveCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BrandGreen).
			Background(BgCard).
			Padding(1, 2)

	// Status Pills & Badges
	BadgeWorking = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(BrandGreen).
			Padding(0, 1)

	BadgeLOS = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(AccentRed).
			Padding(0, 1)

	BadgeOffline = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextSecondary).
			Background(lipgloss.Color("#334155")).
			Padding(0, 1)

	BadgeDyingGasp = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(AccentYellow).
			Padding(0, 1)

	BadgeZTE = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#0284c7")).
			Padding(0, 1)

	BadgeGeneric = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#6366f1")).
			Padding(0, 1)

	// Table Styles
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(BrandEmerald).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(BorderColor).
				Padding(0, 1)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(TextPrimary).
			Padding(0, 1)

	TableAltRowStyle = lipgloss.NewStyle().
				Foreground(TextPrimary).
				Background(lipgloss.Color("#151d2a")).
				Padding(0, 1)

	TableSelectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(TextPrimary).
				Background(lipgloss.Color("#26384f")).
				Padding(0, 1)

	// Footer & Status Bar
	FooterStyle = lipgloss.NewStyle().
			Foreground(TextSecondary).
			Background(lipgloss.Color("#090d16")).
			Padding(0, 1).
			MarginTop(1)

	KeyHelpStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrandEmerald)
)

// RenderProgressBar generates a visual btop-style ASCII gauge
func RenderProgressBar(val, max int, barWidth int) string {
	if max <= 0 {
		max = 1
	}
	ratio := float64(val) / float64(max)
	if ratio > 1.0 {
		ratio = 1.0
	}
	filled := int(ratio * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	if empty < 0 {
		empty = 0
	}

	color := BrandEmerald
	if ratio >= 0.85 {
		color = AccentRed
	} else if ratio >= 0.70 {
		color = AccentYellow
	}

	fillStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", empty)

	return lipgloss.NewStyle().Foreground(color).Render(fillStr) + lipgloss.NewStyle().Foreground(TextMuted).Render(emptyStr)
}

// RenderTopNav renders the Posting-style persistent top navigation tab bar
func RenderTopNav(activeState ViewState, sessName, host, port string, uncfgCount int, width int) string {
	tabs := []struct {
		id    ViewState
		title string
		key   string
	}{
		{StateDashboard, "📊 Dashboard", "1"},
		{StateONUList, "🌐 All ONUs (Explorer)", "2"},
		{StateActivation, "⚡ Add ONU", "3"},
		{StateSessions, "🔌 Sesi OLT", "4"},
	}

	var tabItems []string
	for _, t := range tabs {
		title := t.title
		if t.id == StateActivation && uncfgCount > 0 {
			title = fmt.Sprintf("⚡ Add ONU (%d)", uncfgCount)
		}
		if activeState == t.id {
			tabItems = append(tabItems, ActiveTabStyle.Render(fmt.Sprintf("[%s] %s", t.key, title)))
		} else {
			tabItems = append(tabItems, InactiveTabStyle.Render(fmt.Sprintf("[%s] %s", t.key, title)))
		}
	}

	left := strings.Join(tabItems, " ")

	right := ""
	if sessName != "" {
		statusDot := "🟢"
		connBadge := lipgloss.NewStyle().Bold(true).Foreground(TextPrimary).Background(BrandGreenDark).Padding(0, 1).
			Render(fmt.Sprintf("%s %s (%s:%s)", statusDot, sessName, host, port))
		right = connBadge
	}

	totalLen := lipgloss.Width(left) + lipgloss.Width(right)
	space := width - totalLen - 2
	if space < 1 {
		space = 1
	}

	return left + strings.Repeat(" ", space) + right + "\n\n"
}
