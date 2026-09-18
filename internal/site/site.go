// Package site orchestrates the build.
package site

import "errors"

type Options struct {
	Root string // repository root
	Out  string // output directory (dist)
}

// ErrAuthorInputs wraps every "⟨placeholder⟩ still present" failure, from site.yaml or content/. Exit code 3.
var ErrAuthorInputs = errors.New("author inputs missing")
