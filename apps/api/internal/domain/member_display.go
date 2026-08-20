package domain

import "strings"

// DevDisplayName returns a friendly label for known local dev subs.
func DevDisplayName(sub string) string {
	switch strings.TrimSpace(sub) {
	case "demo-user-a":
		return "Demo User A"
	case "demo-user-b":
		return "Demo User B"
	default:
		return ""
	}
}
