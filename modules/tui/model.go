//go:build tui
// +build tui

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── styles ──────────────────────────────────────────────────────────────

var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63")).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("63"))

	greenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	redStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("63")).
				Padding(0, 1)
)

// ── model ───────────────────────────────────────────────────────────────

type TickMsg time.Time

type model struct {
	collector *StatsCollector
	viewport  viewport.Model
	provider  string
	modelName string
	port      int
	width     int
	height    int
	running   bool
}

func (m model) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.running = false
			return m, tea.Quit
		case "r":
			// Force refresh
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 16
		return m, nil

	case TickMsg:
		m.viewport.SetContent(m.renderContent())
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})
	}

	return m, nil
}

func (m model) View() string {
	if !m.running {
		return ""
	}

	snap := m.collector.Snapshot()

	header := titleStyle.Render("╭──────────────────────────────────────────────╮") + "\n" +
		titleStyle.Render("│") + "  AI Switch Dashboard" +
		strings.Repeat(" ", m.width-60) +
		dimStyle.Render("v0.1.0-beta.1") + "\n" +
		titleStyle.Render("╰──────────────────────────────────────────────╯") + "\n\n"

	// Provider info
	statusIcon := greenStyle.Render("● Running")
	info := headerStyle.Render("Provider:") + " " + m.provider +
		"   " + headerStyle.Render("Model:") + " " + m.modelName + "\n" +
		headerStyle.Render("Port:") + fmt.Sprintf(" %d", m.port) +
		"        " + headerStyle.Render("Status:") + " " + statusIcon + "\n"

	// Stats
	uptime := formatDuration(snap.Uptime)
	stats := dimStyle.Render("────────────────────────────────────────────────") + "\n" +
		headerStyle.Render("📊 Stats") + "  " + dimStyle.Render("Uptime: "+uptime) + "\n" +
		fmt.Sprintf("  Requests: %-10s  Tokens In:   %-10s\n",
			formatInt(snap.TotalReqs), formatInt(snap.TotalPrompt)) +
		fmt.Sprintf("  Tokens Out: %-10s  Total Cost:  $%.6f\n",
			formatInt(snap.TotalCompletion), snap.TotalCost) + "\n"

	// Recent requests table
	tableLines := []string{
		dimStyle.Render("────────────────────────────────────────────────"),
		headerStyle.Render("📝 Recent Requests"),
		tableHeaderStyle.Render(fmt.Sprintf("  %-12s %-20s %-6s %-10s %-8s %-8s",
			"time", "model", "status", "cost", "tokens", "duration")),
	}
	for i, r := range snap.Recent {
		if i >= 50 {
			break
		}
		statusStr := fmt.Sprintf("%d", r.Status)
		statusStyle := greenStyle
		if r.Status >= 400 {
			statusStyle = redStyle
		}
		tableLines = append(tableLines, fmt.Sprintf("  %-12s %-20s %s %-10s %-8s %-8s",
			r.Time.Format("15:04:05"),
			truncate(r.Model, 20),
			statusStyle.Render(statusStr),
			fmt.Sprintf("$%.6f", r.CostUSD),
			fmt.Sprintf("%d/%d", r.PromptTokens, r.CompletionTokens),
			formatDuration(time.Duration(r.DurationMS)*time.Millisecond),
		))
	}

	// Help
	help := "\n" + dimStyle.Render("────────────────────────────────────────────────") + "\n" +
		dimStyle.Render("  q quit  │  r refresh") + "\n"

	return header + info + stats + strings.Join(tableLines, "\n") + help
}

func (m model) renderContent() string {
	return m.View()
}

// ── helpers ─────────────────────────────────────────────────────────────

func formatInt(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
