package attach

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// privateKeyCandidates returns the standard SSH private key paths to try.
func privateKeyCandidates() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(homeDir, ".ssh", "id_ed25519"),
		filepath.Join(homeDir, ".ssh", "id_ed25519_sk"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa_sk"),
		filepath.Join(homeDir, ".ssh", "id_rsa"),
	}
}

// DiscoverAuthMethods returns all available SSH authentication methods.
// It checks the SSH agent first, then tries local private key files.
func DiscoverAuthMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try SSH agent first.
	if m := trySSHAgent(); m != nil {
		methods = append(methods, m)
	}

	// Try local key files.
	for _, path := range privateKeyCandidates() {
		m, err := tryKeyFile(path)
		if err != nil {
			continue
		}
		methods = append(methods, m)
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods available; add a key to ~/.ssh/ or start ssh-agent")
	}

	return methods, nil
}

// trySSHAgent attempts to connect to the SSH agent via SSH_AUTH_SOCK.
func trySSHAgent() ssh.AuthMethod {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil
	}
	conn, err := net.Dial("unix", sock)
	if err != nil {
		return nil
	}
	// The agent client keeps the connection open; it will be garbage collected
	// when the auth method is no longer referenced.
	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

// tryKeyFile reads and parses a private key file. If the key is passphrase-
// protected, it prompts the user for the passphrase.
func tryKeyFile(path string) (ssh.AuthMethod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(data)
	if err != nil {
		// Check if the key is passphrase-protected.
		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			signer, err = parseWithPassphrase(path, data)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	return ssh.PublicKeys(signer), nil
}

// parseWithPassphrase prompts for a passphrase and parses the key.
func parseWithPassphrase(path string, data []byte) (ssh.Signer, error) {
	fmt.Fprintf(os.Stderr, "Enter passphrase for %s: ", path)
	passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("reading passphrase: %w", err)
	}
	return ssh.ParsePrivateKeyWithPassphrase(data, passphrase)
}
