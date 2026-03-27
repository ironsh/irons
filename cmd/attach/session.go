package attach

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ironsh/irons/api"
	"golang.org/x/term"
)

// Config holds everything needed to run an attach session.
type Config struct {
	Host      string
	Port      int
	Username  string
	Command   string
	VMID      string
	AgentName string
	Client    *api.Client
}

// Run is the main entry point for the attach session. It establishes an SSH
// connection, starts the remote command, and runs the CC/egress mode loop.
func Run(cfg Config) error {
	// Discover SSH auth methods.
	authMethods, err := DiscoverAuthMethods()
	if err != nil {
		return err
	}

	// Establish SSH connection.
	conn, err := Dial(cfg.Host, cfg.Port, cfg.Username, authMethods)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Get terminal size and request PTY. Reserve the bottom row for the
	// status bar by giving the remote PTY one fewer row.
	fd := int(os.Stdin.Fd())
	cols, rows, err := term.GetSize(fd)
	if err != nil {
		return fmt.Errorf("getting terminal size: %w", err)
	}
	if err := conn.RequestPTY(rows-1, cols); err != nil {
		return fmt.Errorf("requesting PTY: %w", err)
	}

	// Start remote command.
	if err := conn.Start(cfg.Command); err != nil {
		return fmt.Errorf("starting remote command: %w", err)
	}

	// Save terminal state and enter raw mode.
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("entering raw mode: %w", err)
	}
	defer func() {
		// Restore terminal: termios flags, show cursor, clear status bar
		// line, and reset any leftover ANSI state.
		term.Restore(fd, oldState)
		// Show cursor, exit alt screen, reset attributes, disable mouse
		// tracking modes that tmux/vim may have left enabled.
		os.Stdout.WriteString("\x1b[?25h\x1b[?1049l\x1b[0m" +
			"\x1b[?1000l\x1b[?1002l\x1b[?1003l\x1b[?1006l" +
			"\n")
	}()

	// Start background egress poller.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := NewEgressStore()
	poller := NewPoller(cfg.Client, cfg.VMID, store)
	go poller.Start(ctx)

	// Set up input multiplexer, status bar, and output proxy.
	inputMux := NewInputMux(conn.Stdin)
	defer inputMux.Close()
	go inputMux.Run()

	statusBar := NewStatusBar(rows, cols, &inputMux.mode)
	outputProxy := NewOutputProxy(conn.Stdout, &inputMux.mode, statusBar)
	go outputProxy.Run()

	// Refresh the status bar when the poller has new data, and periodically
	// to keep the "last seen" timestamps fresh.
	go func() {
		pollSub := poller.Subscribe()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-pollSub:
				statusBar.Update(store.Snapshot())
				statusBar.Render()
			case <-ticker.C:
				statusBar.Render()
			}
		}
	}()

	// Handle SIGWINCH for terminal resize.
	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)
	defer signal.Stop(sigwinch)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigwinch:
				if c, r, err := term.GetSize(fd); err == nil {
					conn.Resize(r-1, c)
					statusBar.Resize(r, c)
					statusBar.Render()
				}
			}
		}
	}()

	// Wait for the remote session to finish in a goroutine.
	sshDone := make(chan error, 1)
	go func() {
		sshDone <- conn.Wait()
	}()

	// Mode loop.
	for {
		select {
		case <-outputProxy.Done():
			// Remote side closed.
			return nil

		case <-inputMux.Done():
			// Local stdin closed.
			return nil

		case <-inputMux.QuitCh():
			// Double Ctrl-C — exit session.
			return nil

		case err := <-sshDone:
			// Remote command exited.
			if err != nil {
				return fmt.Errorf("remote session ended: %w", err)
			}
			return nil

		case newMode := <-inputMux.SwitchCh():
			if newMode == modeEgress {
				statusBar.Clear()
				statusBar.AckAlert()
				quit, err := runEgressMode(fd, conn, cfg, store, poller, inputMux)
				if err != nil {
					return err
				}
				if quit {
					return nil
				}
				statusBar.Render()
			}
		}
	}
}

// runEgressMode switches to the bubbletea TUI, then returns when the user
// switches back to CC mode or quits. Returns (true, nil) if the user pressed q.
func runEgressMode(fd int, conn *SSHConn, cfg Config, store *EgressStore, poller *Poller, inputMux *InputMux) (bool, error) {
	// Terminal stays in raw mode — bubbletea handles rendering via stdout
	// escape sequences. Restoring to cooked mode would enable terminal echo,
	// causing keystrokes to appear as raw text in the TUI.
	inputMux.SetMode(modeEgress)

	// Clear the screen instead of using alt screen. Alt screen doesn't nest,
	// and the remote tmux already has us in alt screen — entering/exiting a
	// second alt screen would restore the pre-SSH main buffer on exit.
	os.Stdout.WriteString("\x1b[2J\x1b[H")

	model := NewEgressModel(EgressModelConfig{
		Store:     store,
		Client:    cfg.Client,
		Poller:    poller,
		VMID:      cfg.VMID,
		AgentName: cfg.AgentName,
		SwitchCh:  inputMux.switchCh,
		PollSub:   poller.Subscribe(),
	})

	p := tea.NewProgram(
		model,
		tea.WithInput(inputMux.EgressReader()),
	)

	if _, err := p.Run(); err != nil {
		inputMux.SetMode(modeCC)
		return false, fmt.Errorf("egress monitor: %w", err)
	}

	inputMux.SetMode(modeCC)

	if model.Quitting() {
		return true, nil
	}

	// Force the remote tmux to redraw by toggling the PTY size. Clear the
	// screen first so stale egress TUI content doesn't linger.
	os.Stdout.WriteString("\x1b[2J\x1b[H")
	if cols, rows, err := term.GetSize(fd); err == nil {
		conn.Resize(rows-2, cols)
		conn.Resize(rows-1, cols)
	}

	return false, nil
}
