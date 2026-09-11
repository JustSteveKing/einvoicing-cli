package api

import (
	"fmt"
	"strings"
)

// Problem is an RFC 9457 error from the API. Every error the API returns is
// one, and `Type` is the stable part to branch on.
type Problem struct {
	Type              string              `json:"type"`
	Title             string              `json:"title"`
	Status            int                 `json:"status"`
	Detail            string              `json:"detail"`
	Errors            map[string][]string `json:"errors"`
	Findings          []Finding           `json:"findings"`
	AttemptsRemaining *int                `json:"attempts_remaining"`
	RetryAfter        string              `json:"-"`
}

func (p *Problem) Error() string {
	if p.Detail == "" {
		return p.Title
	}
	return fmt.Sprintf("%s %s", p.Title, p.Detail)
}

// IsType reports whether the problem is of the given type, e.g. "invalid-code",
// matching the last segment of the type URI.
func (p *Problem) IsType(kind string) bool {
	return strings.HasSuffix(p.Type, "/"+kind)
}
