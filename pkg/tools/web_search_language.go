package tools

import (
	"strings"
	"sync"
)

var (
	preferredWebSearchLanguageMu sync.RWMutex
	preferredWebSearchLanguage   string
)

// SetPreferredWebSearchLanguage stores a UI-selected language hint for web search.
func SetPreferredWebSearchLanguage(language string) {
	preferredWebSearchLanguageMu.Lock()
	defer preferredWebSearchLanguageMu.Unlock()
	preferredWebSearchLanguage = strings.TrimSpace(strings.ToLower(language))
}

// GetPreferredWebSearchLanguage returns the current UI-selected language hint.
func GetPreferredWebSearchLanguage() string {
	preferredWebSearchLanguageMu.RLock()
	defer preferredWebSearchLanguageMu.RUnlock()
	return preferredWebSearchLanguage
}
