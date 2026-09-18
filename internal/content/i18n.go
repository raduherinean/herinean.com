package content

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

var Langs = []string{"en", "ro"}

// loadStrings reads <dir>/<lang>.yaml for every language and requires identical key sets.
func loadStrings(dir string) (map[string]map[string]string, Problems) {
	var probs Problems
	out := map[string]map[string]string{}
	for _, lang := range Langs {
		file := filepath.Join(dir, lang+".yaml")
		b, err := os.ReadFile(file)
		if err != nil {
			probs.Add(file, 0, "%v", err)
			continue
		}
		m := map[string]string{}
		if err := yaml.Unmarshal(b, &m); err != nil {
			probs.Add(file, 0, "%v", err)
			continue
		}
		out[lang] = m
	}
	if len(out) != len(Langs) {
		return out, probs
	}
	all := map[string]bool{}
	for _, m := range out {
		for k := range m {
			all[k] = true
		}
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, lang := range Langs {
		for _, k := range keys {
			if v, ok := out[lang][k]; !ok || v == "" {
				probs.Add(filepath.Join(dir, lang+".yaml"), 0, "missing string %q", k)
			}
		}
	}
	return out, probs
}
