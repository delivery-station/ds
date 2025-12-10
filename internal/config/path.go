package config

import (
	"strings"
	"sync"
)

var (
	configPathMu       sync.RWMutex
	explicitConfigFile string
)

// SetExplicitConfigFile records the highest-precedence config file path that
// the loader should merge on top of the default config sources. An empty string
// clears the override.
func SetExplicitConfigFile(path string) {
	configPathMu.Lock()
	explicitConfigFile = strings.TrimSpace(path)
	configPathMu.Unlock()
}

func getExplicitConfigFile() string {
	configPathMu.RLock()
	defer configPathMu.RUnlock()
	return explicitConfigFile
}
