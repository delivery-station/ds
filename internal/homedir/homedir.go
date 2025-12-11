package homedir

import (
	"os"
	"strings"
)

// Provider abstracts how the home directory is resolved.
type Provider interface {
	HomeDir() (string, error)
}

// OSProvider resolves the home directory using the OS and respects HOME when set.
type OSProvider struct{}

func (OSProvider) HomeDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home, nil
	}
	return os.UserHomeDir()
}

// Resolve returns a trimmed home directory path for the given provider, or an empty string on error.
func Resolve(p Provider) string {
	if p == nil {
		p = OSProvider{}
	}

	home, err := p.HomeDir()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(home)
}
