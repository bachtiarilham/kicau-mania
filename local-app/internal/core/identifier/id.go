// Package identifier checks the syntax of application identifiers.
package identifier

import "regexp"

var pattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func Valid(s string) bool { return pattern.MatchString(s) }
