package errorutil

import (
	"encoding/json"
	"fmt"
	"strings"
)

const unknownError = "unknown error"

// MessageWithDetails composes an error message from the provider message, detail payload, and fallback type.
func MessageWithDetails(message, fallbackType string, detail json.RawMessage) string {
	msg := strings.TrimSpace(message)
	details := strings.TrimSpace(string(detail))

	switch {
	case msg != "" && details != "":
		return fmt.Sprintf("%s: %s", msg, details)
	case msg != "":
		return msg
	case details != "":
		return details
	case fallbackType != "":
		return fallbackType
	default:
		return unknownError
	}
}
