// Command site builds herinean.com: content/ + assets/ + templates/ + i18n/ → dist/.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	_ "time/tzdata" // Europe/Bucharest travels with the binary

	"github.com/raduherinean/herinean.com/internal/site"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "build":
		err = site.Build(site.Options{Root: ".", Out: "dist"})
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		dist := fs.Bool("dist", false, "check the built dist/ instead of the sources")
		draft := fs.Bool("draft", false, "blank title/date/pillar/summary are warnings, not failures (piece/* branches)")
		_ = fs.Parse(os.Args[2:])
		switch {
		case *dist && *draft:
			fmt.Fprintln(os.Stderr, "site check: --dist and --draft do not combine")
			os.Exit(2)
		case *dist:
			err = site.CheckDist(site.Options{Root: ".", Out: "dist"})
		default:
			err = site.Check(site.Options{Root: ".", Draft: *draft})
		}
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		static := fs.Bool("static", false, "serve dist/ as built: no rebuild, no draft mode (what CI audits)")
		host := fs.String("host", "127.0.0.1", "bind address")
		port := fs.Int("port", 8080, "port")
		_ = fs.Parse(os.Args[2:])
		err = site.Serve(site.Options{Root: ".", Out: "dist", Static: *static}, *host, *port)
	case "new":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "usage: site new <en|ro> <slug>")
			os.Exit(2)
		}
		err = site.New(site.Options{Root: "."}, os.Args[2], os.Args[3])
	case "scorecard":
		fs := flag.NewFlagSet("scorecard", flag.ExitOnError)
		in := fs.String("in", "scorecard.json", "CI results")
		manual := fs.String("manual", "data/scorecard-manual.yaml", "manual rows")
		out := fs.String("out", "scorecard.html", "HTML fragment")
		_ = fs.Parse(os.Args[2:])
		err = site.Scorecard(site.Options{Root: "."}, *in, *manual, *out)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "site:", err)
		if errors.Is(err, site.ErrAuthorInputs) {
			os.Exit(3) // author inputs missing: the pre-commit hook warns, CI fails
		}
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage:
  site build              content/ + assets/ + templates/ + i18n/ → dist/
  site check [--draft|--dist]  validate sources (--draft: blanks site new leaves are warnings) or the built dist/
  site serve [--static] [--host H] [--port P]
  site new <en|ro> <slug>
  site scorecard --in scorecard.json --manual data/scorecard-manual.yaml --out scorecard.html`)
}
