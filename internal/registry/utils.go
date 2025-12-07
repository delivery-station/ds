package registry

import "strings"

// IsInsecureRegistry returns true when the target registry should be accessed over plain HTTP.
// It compares the normalized registry host (without scheme or trailing slashes) against the
// configured insecure registry list.
func IsInsecureRegistry(target string, insecureList []string) bool {
	normalizedTarget := normalizeRegistrySafe(target)
	if normalizedTarget == "" {
		return false
	}

	targetHost := stripRegistryPath(normalizedTarget)

	for _, entry := range insecureList {
		normalizedEntry := normalizeRegistrySafe(entry)
		if normalizedEntry == "" {
			continue
		}

		if normalizedTarget == normalizedEntry {
			return true
		}

		if targetHost == stripRegistryPath(normalizedEntry) {
			return true
		}
	}

	return false
}

// normalizeRegistrySafe wraps the existing normalizeRegistry helper with additional trimming and
// case normalization for comparisons that should be case-insensitive.
func normalizeRegistrySafe(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	normalized := normalizeRegistry(trimmed)
	return strings.ToLower(normalized)
}

func stripRegistryPath(value string) string {
	if idx := strings.IndexRune(value, '/'); idx != -1 {
		return value[:idx]
	}
	return value
}
