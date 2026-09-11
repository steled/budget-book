package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
)

// parseAmountCents converts a decimal Euro amount (e.g. "12.34") as sent by
// the frontend into an integer cent amount, rounding to avoid floating
// point drift. Negative amounts are rejected: sign is derived from the
// category type, not entered directly.
func parseAmountCents(s string) (int64, error) {
	amount, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid amount %q: %w", s, err)
	}
	if amount < 0 {
		return 0, fmt.Errorf("amount must not be negative")
	}
	return int64(math.Round(amount * 100)), nil
}

// pathID extracts and parses the {id} path value from r.
func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
