package content

import (
	"fmt"
	"strings"
)

// Problem is one validation failure, addressed so an editor can jump to it.
type Problem struct {
	File string
	Line int
	Msg  string
}

func (p Problem) Error() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", p.File, p.Line, p.Msg)
	}
	return fmt.Sprintf("%s: %s", p.File, p.Msg)
}

type Problems []Problem

func (ps Problems) Err() error {
	if len(ps) == 0 {
		return nil
	}
	var b strings.Builder
	for i, p := range ps {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(p.Error())
	}
	return fmt.Errorf("%d problem(s):\n%s", len(ps), b.String())
}

func (ps *Problems) Add(file string, line int, format string, args ...any) {
	*ps = append(*ps, Problem{File: file, Line: line, Msg: fmt.Sprintf(format, args...)})
}
