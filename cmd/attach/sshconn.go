package attach

import (
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// SSHConn wraps a Go SSH client, session, and the session's stdin/stdout pipes.
type SSHConn struct {
	client  *ssh.Client
	session *ssh.Session
	Stdin   io.WriteCloser
	Stdout  io.Reader
}

// Dial establishes an SSH connection and opens a session.
func Dial(host string, port int, username string, authMethods []ssh.AuthMethod) (*SSHConn, error) {
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", addr, err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("opening SSH session: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("getting stdin pipe: %w", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("getting stdout pipe: %w", err)
	}

	return &SSHConn{
		client:  client,
		session: session,
		Stdin:   stdin,
		Stdout:  stdout,
	}, nil
}

// RequestPTY requests a pseudo-terminal on the remote side.
func (c *SSHConn) RequestPTY(rows, cols int) error {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	return c.session.RequestPty("xterm-256color", rows, cols, modes)
}

// Start starts the given command on the remote side.
func (c *SSHConn) Start(command string) error {
	return c.session.Start(command)
}

// Resize sends a window-change request to update the remote PTY size.
func (c *SSHConn) Resize(rows, cols int) error {
	return c.session.WindowChange(rows, cols)
}

// Wait waits for the remote command to exit.
func (c *SSHConn) Wait() error {
	return c.session.Wait()
}

// Close tears down the session and connection.
func (c *SSHConn) Close() {
	if c.session != nil {
		c.session.Close()
	}
	if c.client != nil {
		c.client.Close()
	}
}
