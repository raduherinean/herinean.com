// Package site orchestrates the build.
package site

import "errors"

type Options struct {
	Root  string // repository root
	Out   string // output directory (dist)
	Draft bool   // serve only: render pieces whose title/date/pillar/summary are still blank, with visible defaults
}

// ErrAuthorInputs wraps every "⟨placeholder⟩ still present" failure, from site.yaml or content/. Exit code 3.
var ErrAuthorInputs = errors.New("author inputs missing")
