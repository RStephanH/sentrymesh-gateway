package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tab int

const (
	tabOverview tab = iota
	tabActivity
)

var tabNames = []string{"Overview", "Activity"}

var (
	activeTabStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 2)
	inactiveTabStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(0, 2)

	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	invalidStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	throttleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	replayStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	floodStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
)

type Model struct {
	events chan Event

	activeTab tab
	ready     bool

	totalTelemetry int
	alertCounts    map[string]int

	logLines []string
	viewport viewport.Model
}

func NewModel(events chan Event) Model {
	return Model{
		events:      events,
		alertCounts: make(map[string]int),
	}
}

func (m Model) Init() tea.Cmd {
	return waitForEvent(m.events)
}

func waitForEvent(events chan Event) tea.Cmd {
	return func() tea.Msg {
		return <-events
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch e := msg.(type) {

	case tea.WindowSizeMsg:
		const headerHeight, footerHeight = 3, 2
		if !m.ready {
			m.viewport = viewport.New(e.Width, e.Height-headerHeight-footerHeight)
			m.ready = true
		} else {
			m.viewport.Width = e.Width
			m.viewport.Height = e.Height - headerHeight - footerHeight
		}
		m.viewport.SetContent(strings.Join(m.logLines, "\n"))
		return m, nil

	case TelemetryEvent:
		m.totalTelemetry++
		m.appendLog(okStyle, "OK      ",
			fmt.Sprintf("[%s] device=%-10s temp=%.1f hum=%.1f",
				e.ReceivedAt.Format("15:04:05"), e.DeviceID, e.Temperature, e.Humidity))
		return m, waitForEvent(m.events)

	case InvalidPayloadEvent:
		m.alertCounts["invalid_payload"]++
		m.appendLog(invalidStyle, "INVALID ",
			fmt.Sprintf("[%s] device=%-10s %s",
				e.OccurredAt.Format("15:04:05"), e.DeviceID, e.Err))
		return m, waitForEvent(m.events)

	case ReplayEvent:
		m.alertCounts["replay"]++
		m.appendLog(replayStyle, "REPLAY  ",
			fmt.Sprintf("[%s] device=%-10s timestamp=%d",
				e.OccurredAt.Format("15:04:05"), e.DeviceID, e.Timestamp))
		return m, waitForEvent(m.events)

	case RateLimitEvent:
		m.alertCounts["rate_limit"]++
		m.appendLog(throttleStyle, "THROTTLE",
			fmt.Sprintf("[%s] device=%-10s",
				e.OccurredAt.Format("15:04:05"), e.DeviceID))
		return m, waitForEvent(m.events)

	case FloodEvent:
		m.alertCounts["flood"]++
		m.appendLog(floodStyle, "FLOOD!! ",
			fmt.Sprintf("[%s] device=%-10s window=%s threshold=%d",
				e.OccurredAt.Format("15:04:05"), e.DeviceID, e.Window, e.Threshold))
		return m, waitForEvent(m.events)

	case tea.KeyMsg:
		switch e.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "shift+tab":
			m.activeTab = (m.activeTab + 1) % tab(len(tabNames))
			return m, nil
		}
		if m.activeTab == tabActivity {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(e)
			return m, cmd
		}
	}

	return m, nil
}

// appendLog renders the label in its severity color, appends the full
// line to history (capped to avoid unbounded memory over a long demo),
// and keeps the viewport synced and scrolled to the newest entry.
func (m *Model) appendLog(style lipgloss.Style, label, rest string) {
	line := fmt.Sprintf("%s %s", style.Render(label), rest)
	m.logLines = append(m.logLines, line)

	const maxLines = 500
	if len(m.logLines) > maxLines {
		m.logLines = m.logLines[len(m.logLines)-maxLines:]
	}
	if m.ready {
		m.viewport.SetContent(strings.Join(m.logLines, "\n"))
		m.viewport.GotoBottom()
	}
}

func (m Model) renderTabBar() string {
	var b strings.Builder
	for i, name := range tabNames {
		if tab(i) == m.activeTab {
			b.WriteString(activeTabStyle.Render(name))
		} else {
			b.WriteString(inactiveTabStyle.Render(name))
		}
	}
	return b.String()
}

func (m Model) View() string {
	if !m.ready {
		return "Loading...\n"
	}

	var b strings.Builder
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	switch m.activeTab {
	case tabOverview:
		b.WriteString(fmt.Sprintf("Telemetry accepted: %d\n\n", m.totalTelemetry))
		b.WriteString("Alerts\n")
		b.WriteString(fmt.Sprintf("  %-25s %d\n", invalidStyle.Render("invalid_payload"), m.alertCounts["invalid_payload"]))
		b.WriteString(fmt.Sprintf("  %-25s %d\n", replayStyle.Render("replay"), m.alertCounts["replay"]))
		b.WriteString(fmt.Sprintf("  %-25s %d\n", throttleStyle.Render("rate_limit"), m.alertCounts["rate_limit"]))
		b.WriteString(fmt.Sprintf("  %-25s %d\n", floodStyle.Render("flood"), m.alertCounts["flood"]))
		b.WriteString("\n\nTab: switch view · q: quit\n")

	case tabActivity:
		b.WriteString(m.viewport.View())
		b.WriteString("\n\nTab: switch view · ↑/↓: scroll · q: quit\n")
	}

	return b.String()
}
