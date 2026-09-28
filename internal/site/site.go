// Package site orchestrates the build.
package site

import "errors"

type Options struct {
	Root   string // repository root
	Out    string // output directory (dist)
	Draft  bool   // serve and check --draft: blank title/date/pillar/summary are warnings with visible defaults, not problems
	Static bool   // serve only: serve dist/ as built — no rebuild, no draft mode; what CI audits is what deploys
}

// ErrAuthorInputs wraps every "⟨placeholder⟩ still present" failure, from site.yaml or content/. Exit code 3.
var ErrAuthorInputs = errors.New("author inputs missing")
