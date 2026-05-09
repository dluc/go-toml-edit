package tomledit

import "fmt"

// ParseError represents a lexing or parsing error with position information.
type ParseError struct {
	Line    int
	Column  int
	Offset  int
	Snippet string
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Message)
}
