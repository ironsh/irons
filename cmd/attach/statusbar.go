package attach

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"unicode/utf8"
)

// StatusBar renders a persistent status line on the bottom row of the terminal
// during CC (SSH) mode. It uses ANSI escape sequences to position itself
// without disturbing the SSH output above.
type StatusBar struct {
	mu   sync.Mutex
	rows int32 // current terminal height (atomic for resize)
	cols int32
	mode *atomic.Int32

	// Egress state.
	allowedCount int
	deniedCount  int
	warnCount    int
	newDeniedHost string          // non-empty when there's an unseen denied host to alert on
	seenDenied    map[string]bool // hosts the user has already been alerted about
}

// NewStatusBar creates a status bar. rows/cols are the full terminal dimensions.
func NewStatusBar(rows, cols int, mode *atomic.Int32) *StatusBar {
	return &StatusBar{
		rows:       int32(rows),
		cols:       int32(cols),
		mode:       mode,
		seenDenied: make(map[string]bool),
	}
}

// Resize updates the terminal dimensions.
func (s *StatusBar) Resize(rows, cols int) {
	atomic.StoreInt32(&s.rows, int32(rows))
	atomic.StoreInt32(&s.cols, int32(cols))
}

// Update refreshes the status bar's data from an egress store snapshot.
func (s *StatusBar) Update(domains []DomainEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	allowed := 0
	denied := 0
	warned := 0

	for _, d := range domains {
		switch d.Verdict {
		case "allowed":
			allowed++
		case "warn":
			warned++
		default: // "blocked", "denied", etc.
			denied++
			// Only alert on hosts we haven't flagged before.
			if !s.seenDenied[d.Host] {
				s.seenDenied[d.Host] = true
				s.newDeniedHost = d.Host
			}
		}
	}

	s.allowedCount = allowed
	s.deniedCount = denied
	s.warnCount = warned
}

// AckAlert clears the new-denied alert state (e.g. after the user toggles
// to the egress monitor and sees the denied hosts).
func (s *StatusBar) AckAlert() {
	s.mu.Lock()
	s.newDeniedHost = ""
	s.mu.Unlock()
}

// Render writes the status bar to stdout using ANSI escape sequences.
// Safe to call from any goroutine; only renders in CC mode.
func (s *StatusBar) Render() {
	if s.mode.Load() != modeCC {
		return
	}

	s.mu.Lock()
	allowed := s.allowedCount
	denied := s.deniedCount
	warned := s.warnCount
	alertHost := s.newDeniedHost
	s.mu.Unlock()

	rows := atomic.LoadInt32(&s.rows)
	cols := int(atomic.LoadInt32(&s.cols))

	// Right side: always show the egress monitor hint.
	right := " Ctrl-] egress monitor "

	// Left side: counts + optional alert.
	left := fmt.Sprintf(" %d allowed", allowed)
	if denied > 0 {
		left += fmt.Sprintf(" / %d denied", denied)
	}
	if warned > 0 {
		left += fmt.Sprintf(" / %d warned", warned)
	}

	var alertPart string
	if alertHost != "" {
		alertPart = fmt.Sprintf(" DENIED %s ", alertHost)
	}

	// Calculate display widths (rune count, not byte count).
	leftWidth := utf8.RuneCountInString(left)
	rightWidth := utf8.RuneCountInString(right)
	alertWidth := utf8.RuneCountInString(alertPart)

	// Build the bar: [left] [padding or alert] [right]
	// The alert sits between left and right, highlighted in red.
	middleWidth := cols - leftWidth - rightWidth
	if middleWidth < 0 {
		middleWidth = 0
	}

	// Normal style: gray on dark bg.
	const normalOn = "\x1b[38;5;248;48;5;236m"
	// Alert style: white bold on red bg.
	const alertOn = "\x1b[1;37;41m"
	const styleOff = "\x1b[0m"

	// Position cursor at the status bar row.
	preamble := fmt.Sprintf("\x1b[s\x1b[%d;1H\x1b[2K", rows)

	var bar string
	if alertPart != "" && alertWidth+1 <= middleWidth {
		// Show alert in red between the counts and the hint,
		// with a normal-colored space before it for breathing room.
		pad := middleWidth - alertWidth - 1
		bar = preamble +
			normalOn + left + " " + styleOff +
			alertOn + alertPart + styleOff +
			normalOn + spaces(pad) + right + styleOff +
			"\x1b[u"
	} else {
		// No alert or no room for it — just pad.
		bar = preamble +
			normalOn + left + spaces(middleWidth) + right + styleOff +
			"\x1b[u"
	}

	os.Stdout.WriteString(bar)
}

// Clear removes the status bar from the terminal.
func (s *StatusBar) Clear() {
	rows := atomic.LoadInt32(&s.rows)
	fmt.Fprintf(os.Stdout, "\x1b[s\x1b[%d;1H\x1b[2K\x1b[u", rows)
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
