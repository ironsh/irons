package attach

import (
	"io"
	"os"
	"sync/atomic"
	"time"
)

const ctrlRightBracket = 0x1d // Ctrl-]
const ctrlC            = 0x03 // Ctrl-C

const doublePressWindow = 1 * time.Second

// modeCC and modeEgress are the two operating modes.
const (
	modeCC     int32 = 0
	modeEgress int32 = 1
)

// InputMux owns os.Stdin reads and routes bytes to either the SSH channel
// (CC mode) or a pipe feeding bubbletea (egress mode).
type InputMux struct {
	mode     atomic.Int32
	sshStdin io.Writer
	// egressW is written to in egress mode; the read end feeds bubbletea.
	egressR *io.PipeReader
	egressW *io.PipeWriter
	// switchCh signals that Ctrl-] was pressed. The value is the new mode.
	switchCh chan int32
	// quitCh signals that double Ctrl-C was pressed (exit session).
	quitCh chan struct{}
	// done is closed when stdin hits EOF or an error.
	done chan struct{}
	// lastCtrlC tracks the timestamp of the last Ctrl-C for double-press detection.
	lastCtrlC time.Time
}

// NewInputMux creates an InputMux. The returned PipeReader should be passed
// to bubbletea via tea.WithInput().
func NewInputMux(sshStdin io.Writer) *InputMux {
	pr, pw := io.Pipe()
	m := &InputMux{
		sshStdin: sshStdin,
		egressR:  pr,
		egressW:  pw,
		switchCh: make(chan int32, 1),
		quitCh:   make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
	m.mode.Store(modeCC)
	return m
}

// EgressReader returns the read end of the pipe for bubbletea input.
func (m *InputMux) EgressReader() *io.PipeReader {
	return m.egressR
}

// SwitchCh returns the channel that signals mode switches.
func (m *InputMux) SwitchCh() <-chan int32 {
	return m.switchCh
}

// QuitCh returns a channel that signals the user wants to exit (double Ctrl-C).
func (m *InputMux) QuitCh() <-chan struct{} {
	return m.quitCh
}

// Done returns a channel that is closed when stdin reading stops.
func (m *InputMux) Done() <-chan struct{} {
	return m.done
}

// SetMode changes the current routing mode.
func (m *InputMux) SetMode(mode int32) {
	m.mode.Store(mode)
}

// Run reads from os.Stdin in a loop, routing bytes based on the current mode.
// It blocks until stdin returns an error (EOF, closed, etc).
//
// In CC mode, Ctrl-] (0x1d) is intercepted and triggers a mode switch signal.
// In egress mode, all bytes (including Ctrl-]) pass through to bubbletea's
// input pipe so the TUI can handle keys natively.
func (m *InputMux) Run() {
	defer close(m.done)

	buf := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			current := m.mode.Load()
			if current == modeCC {
				m.forwardCC(buf[:n])
			} else {
				// In egress mode, pass everything to bubbletea unchanged.
				m.egressW.Write(buf[:n])
			}
		}
		if err != nil {
			return
		}
	}
}

// forwardCC processes a chunk of bytes in CC mode. It scans for two special
// sequences:
//   - Ctrl-] (0x1d): triggers a mode switch to the egress monitor.
//   - Double Ctrl-C (0x03) within 500ms: first Ctrl-C is forwarded to SSH,
//     second signals session exit.
//
// All other bytes are forwarded to SSH unchanged.
func (m *InputMux) forwardCC(data []byte) {
	for len(data) > 0 {
		// Find the next special byte.
		idx := -1
		for i, b := range data {
			if b == ctrlRightBracket || b == ctrlC {
				idx = i
				break
			}
		}

		if idx == -1 {
			m.sshStdin.Write(data)
			return
		}

		// Forward bytes before the special byte.
		if idx > 0 {
			m.sshStdin.Write(data[:idx])
		}

		special := data[idx]
		data = data[idx+1:]

		switch special {
		case ctrlRightBracket:
			select {
			case m.switchCh <- modeEgress:
			default:
			}

		case ctrlC:
			now := time.Now()
			if !m.lastCtrlC.IsZero() && now.Sub(m.lastCtrlC) < doublePressWindow {
				// Double Ctrl-C — signal quit.
				m.lastCtrlC = time.Time{}
				select {
				case m.quitCh <- struct{}{}:
				default:
				}
			} else {
				// First Ctrl-C — forward to SSH and start the window.
				m.lastCtrlC = now
				m.sshStdin.Write([]byte{ctrlC})
			}
		}
	}
}

// Close cleans up the pipe.
func (m *InputMux) Close() {
	m.egressW.Close()
	m.egressR.Close()
}

// OutputProxy copies SSH stdout to os.Stdout, respecting a pause flag.
// When paused (egress mode), it discards output to prevent SSH channel backpressure.
// After each write in CC mode, it re-renders the status bar.
type OutputProxy struct {
	mode      *atomic.Int32
	source    io.Reader
	statusBar *StatusBar
	done      chan struct{}
}

// NewOutputProxy creates an OutputProxy.
func NewOutputProxy(source io.Reader, mode *atomic.Int32, statusBar *StatusBar) *OutputProxy {
	return &OutputProxy{
		source:    source,
		mode:      mode,
		statusBar: statusBar,
		done:      make(chan struct{}),
	}
}

// Done returns a channel closed when the output proxy stops (remote EOF).
func (o *OutputProxy) Done() <-chan struct{} {
	return o.done
}

// Run copies from source to os.Stdout. In egress mode, bytes are discarded.
// Blocks until source returns an error.
func (o *OutputProxy) Run() {
	defer close(o.done)

	buf := make([]byte, 32*1024)
	for {
		n, err := o.source.Read(buf)
		if n > 0 && o.mode.Load() == modeCC {
			os.Stdout.Write(buf[:n])
			o.statusBar.Render()
		}
		if err != nil {
			return
		}
	}
}
