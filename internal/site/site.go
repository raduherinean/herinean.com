// Package site orchestrates the build.
package site

import "errors"

type Options struct {
	Root string // repository root
	Out  string // output directory (dist)
}

var errNotImplemented = errors.New("not implemented yet")

// ErrAuthorInputs wraps every "⟨placeholder⟩ still present" failure, from site.yaml or content/. Exit code 3.
var ErrAuthorInputs = errors.New("author inputs missing")

func Serve(o Options, host string, port int) error      { return errNotImplemented }
func New(o Options, lang, slug string) error            { return errNotImplemented }
func Scorecard(o Options, in, manual, out string) error { return errNotImplemented }
