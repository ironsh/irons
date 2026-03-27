package attach

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/ironsh/irons/api"
)

// EgressModel is the bubbletea model for the egress monitor TUI.
type EgressModel struct {
	store     *EgressStore
	client    *api.Client
	poller    *Poller
	vmID      string
	agentName string

	// UI state.
	domains      []DomainEntry
	cursor       int
	selectedHost string // tracks selection by host name across reorders
	detailMode   bool
	width        int
	height       int

	// Communication.
	switchCh chan<- int32
	quitting bool
	pollSub  <-chan struct{}

	// Transient status message (e.g. error from API call).
	statusMsg string
}

// EgressModelConfig holds the configuration for creating an EgressModel.
type EgressModelConfig struct {
	Store     *EgressStore
	Client    *api.Client
	Poller    *Poller
	VMID      string
	AgentName string
	SwitchCh  chan<- int32
	PollSub   <-chan struct{}
}

// NewEgressModel creates a new egress monitor model.
func NewEgressModel(cfg EgressModelConfig) *EgressModel {
	domains := cfg.Store.Snapshot()
	var selectedHost string
	if len(domains) > 0 {
		selectedHost = domains[0].Host
	}
	return &EgressModel{
		store:        cfg.Store,
		client:       cfg.Client,
		poller:       cfg.Poller,
		vmID:         cfg.VMID,
		agentName:    cfg.AgentName,
		switchCh:     cfg.SwitchCh,
		pollSub:      cfg.PollSub,
		domains:      domains,
		selectedHost: selectedHost,
	}
}

// Quitting returns true if the user pressed q to quit entirely.
func (m *EgressModel) Quitting() bool {
	return m.quitting
}

// Custom messages.

type egressDataMsg struct{}
type egressActionMsg struct{ err error }

func waitForPoll(sub <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-sub
		return egressDataMsg{}
	}
}

func allowDomain(client *api.Client, host string) tea.Cmd {
	return func() tea.Msg {
		_, err := client.EgressCreateRule(api.EgressRuleRequest{
			Name: host,
			Host: host,
		})
		return egressActionMsg{err: err}
	}
}

func denyDomain(client *api.Client, ruleID string) tea.Cmd {
	return func() tea.Msg {
		err := client.EgressDeleteRule(ruleID)
		return egressActionMsg{err: err}
	}
}

// Bubbletea interface.

func (m *EgressModel) Init() tea.Cmd {
	return waitForPoll(m.pollSub)
}

func (m *EgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		return m.handleKey(msg)

	case egressDataMsg:
		m.refreshDomains()
		return m, waitForPoll(m.pollSub)

	case egressActionMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.statusMsg = "Rule updated."
			// Trigger an immediate poll so the store picks up the new rule state.
			m.poller.Refresh()
		}
	}

	return m, nil
}

func (m *EgressModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keyBindings.Quit):
		m.quitting = true
		return m, tea.Quit

	case msg.String() == "ctrl+]":
		// Switch back to CC session.
		select {
		case m.switchCh <- modeCC:
		default:
		}
		return m, tea.Quit

	case key.Matches(msg, keyBindings.Up):
		if m.cursor > 0 {
			m.cursor--
			m.trackSelection()
		}

	case key.Matches(msg, keyBindings.Down):
		if m.cursor < len(m.domains)-1 {
			m.cursor++
			m.trackSelection()
		}

	case key.Matches(msg, keyBindings.Allow):
		if m.cursor < len(m.domains) {
			host := m.domains[m.cursor].Host
			m.statusMsg = fmt.Sprintf("Allowing %s...", host)
			return m, allowDomain(m.client, host)
		}

	case key.Matches(msg, keyBindings.Deny):
		if m.cursor < len(m.domains) {
			d := m.domains[m.cursor]
			ruleID := m.store.RuleIDForHost(d.Host)
			if ruleID == "" {
				m.statusMsg = fmt.Sprintf("No rule to remove for %s", d.Host)
			} else {
				m.statusMsg = fmt.Sprintf("Removing rule for %s...", d.Host)
				return m, denyDomain(m.client, ruleID)
			}
		}

	case key.Matches(msg, keyBindings.Detail):
		m.detailMode = !m.detailMode
		m.statusMsg = ""
	}

	return m, nil
}

// trackSelection updates selectedHost to match the current cursor position.
func (m *EgressModel) trackSelection() {
	if m.cursor >= 0 && m.cursor < len(m.domains) {
		m.selectedHost = m.domains[m.cursor].Host
	}
}

// refreshDomains updates the domain list from the store, preserving the
// selected host across reorders.
func (m *EgressModel) refreshDomains() {
	m.domains = m.store.Snapshot()

	// Find the previously selected host in the new ordering.
	if m.selectedHost != "" {
		for i, d := range m.domains {
			if d.Host == m.selectedHost {
				m.cursor = i
				return
			}
		}
	}

	// Host no longer exists; clamp cursor.
	if m.cursor >= len(m.domains) && len(m.domains) > 0 {
		m.cursor = len(m.domains) - 1
	}
	m.trackSelection()
}

func (m *EgressModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var b strings.Builder

	// Header.
	header := headerStyle.Width(m.width).Render(
		fmt.Sprintf(" Egress Monitor — %s — Ctrl-] to return", m.agentName),
	)
	b.WriteString(header)
	b.WriteString("\n")

	if len(m.domains) == 0 {
		b.WriteString("\n  No egress events yet.\n")
	} else if m.detailMode && m.cursor < len(m.domains) {
		b.WriteString(m.renderDetail())
	} else {
		b.WriteString(m.renderList())
	}

	// Status line.
	if m.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(" " + m.statusMsg))
		b.WriteString("\n")
	}

	// Footer.
	b.WriteString("\n")
	footer := footerStyle.Width(m.width).Render(
		" ↑/↓ navigate  a allow  d deny  i detail  Ctrl-] back  q quit",
	)
	b.WriteString(footer)

	return b.String()
}

func (m *EgressModel) renderList() string {
	var b strings.Builder

	// Column header.
	b.WriteString(colHeaderStyle.Render(
		fmt.Sprintf("  %-40s %6s  %-10s  %-20s", "HOST", "COUNT", "STATUS", "LAST SEEN"),
	))
	b.WriteString("\n")

	// Calculate how many rows we can show.
	maxRows := m.height - 6 // header + col header + footer + status + padding
	if maxRows < 1 {
		maxRows = 1
	}

	// Determine scroll offset to keep cursor visible.
	offset := 0
	if m.cursor >= maxRows {
		offset = m.cursor - maxRows + 1
	}

	end := offset + maxRows
	if end > len(m.domains) {
		end = len(m.domains)
	}

	for i := offset; i < end; i++ {
		d := m.domains[i]
		cursor := "  "
		style := rowStyle
		if i == m.cursor {
			cursor = "> "
			style = selectedRowStyle
		}

		verdict := verdictLabel(d.Verdict)
		lastSeen := formatLastSeen(d.LastSeen)

		line := fmt.Sprintf("%s%-40s %6d  %s  %-20s",
			cursor,
			truncate(d.Host, 40),
			d.RequestCount,
			verdict,
			lastSeen,
		)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *EgressModel) renderDetail() string {
	d := m.domains[m.cursor]
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  Host:     %s\n", d.Host))
	b.WriteString(fmt.Sprintf("  Requests: %d\n", d.RequestCount))
	b.WriteString(fmt.Sprintf("  Status:   %s\n", verdictLabel(d.Verdict)))
	b.WriteString(fmt.Sprintf("  Protocol: %s\n", d.Protocol))
	if d.RuleID != "" {
		b.WriteString(fmt.Sprintf("  Rule ID:  %s\n", d.RuleID))
	}
	b.WriteString("\n  Recent events:\n")

	// Show up to 10 most recent events.
	start := len(d.Events) - 10
	if start < 0 {
		start = 0
	}
	for i := len(d.Events) - 1; i >= start; i-- {
		ev := d.Events[i]
		ts := ev.Timestamp.Local().Format(time.RFC3339)
		v := normalizeVerdict(ev)
		b.WriteString(fmt.Sprintf("    %s  %s  %s\n", ts, verdictLabel(v), ev.Protocol))
	}

	return b.String()
}

// Styles.

var (
	headerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Bold(true)

	footerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("248"))

	colHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("248")).
			Bold(true)

	rowStyle         = lipgloss.NewStyle()
	selectedRowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("237")).
				Bold(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	verdictAllowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	verdictWarnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	verdictDenyStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
)

func verdictLabel(verdict string) string {
	switch verdict {
	case "allowed":
		return verdictAllowStyle.Render("ALLOW ")
	case "warn":
		return verdictWarnStyle.Render("WARN  ")
	default:
		return verdictDenyStyle.Render("DENY  ")
	}
}

func formatLastSeen(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// Key bindings.

type egressKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Allow  key.Binding
	Deny   key.Binding
	Detail key.Binding
	Quit   key.Binding
}

var keyBindings = egressKeyMap{
	Up:     key.NewBinding(key.WithKeys("up", "k")),
	Down:   key.NewBinding(key.WithKeys("down", "j")),
	Allow:  key.NewBinding(key.WithKeys("a")),
	Deny:   key.NewBinding(key.WithKeys("d")),
	Detail: key.NewBinding(key.WithKeys("i")),
	Quit:   key.NewBinding(key.WithKeys("q")),
}
