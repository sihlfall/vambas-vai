package ast

import "sihlfall.ch/vambas-vai/core"

// Comment ranges come from the VBA lexer, including trivia it normally skips.
type Comment struct {
	Range      core.TextRange
	Text       string
	Standalone bool
}

// Annotation is syntax metadata. Target is an existing declaration, not a
// syntax child. Unknown names are retained for other tooling/adapters.
type Annotation struct {
	Name      string
	Arguments []string
	Range     core.TextRange
	Target    *Node
	Error     string
}
