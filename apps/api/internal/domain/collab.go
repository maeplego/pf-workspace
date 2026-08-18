package domain

import (
	"time"

	"github.com/oklog/ulid/v2"
)

const CollabTicketTTL = 15 * time.Minute

// ValidCollabRoom rejects path-like names so the WS server cannot be pointed at arbitrary files.
func ValidCollabRoom(name string) bool {
	if name == "" || len(name) > 32 {
		return false
	}
	_, err := ulid.Parse(name)
	return err == nil
}
