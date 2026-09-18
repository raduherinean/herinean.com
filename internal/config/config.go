// Package config loads site.yaml and owns the URL scheme.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Author struct {
	LinkedIn string `yaml:"linkedin"`
	X        string `yaml:"x"`
	GitHub   string `yaml:"github"`
	Email    string `yaml:"email"`
}

type Config struct {
	BaseURL      string            `yaml:"base_url"`
	Name         string            `yaml:"name"`
	Tagline      map[string]string `yaml:"tagline"`
	Author       Author            `yaml:"author"`
	AIDisclosure map[string]string `yaml:"ai_disclosure"`
}

// Placeholder markers: anything the author still has to fill in. The build refuses to ship them.
const openMark, closeMark = "⟨", "⟩"

// ErrPlaceholder marks "author inputs still missing"; site maps it to exit code 3 so the pre-commit hook warns instead of blocking before launch.
var ErrPlaceholder = errors.New("author placeholder present")

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.validate(); err != nil {
		if errors.Is(err, ErrPlaceholder) {
			return &c, fmt.Errorf("%s: %w", path, err) // the config is usable; site check still runs the other rules
		}
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

func (c *Config) validate() error {
	req := map[string]string{
		"base_url": c.BaseURL, "name": c.Name,
		"tagline.en": c.Tagline["en"], "tagline.ro": c.Tagline["ro"],
		"author.linkedin": c.Author.LinkedIn, "author.x": c.Author.X, "author.github": c.Author.GitHub, "author.email": c.Author.Email,
		"ai_disclosure.en": c.AIDisclosure["en"], "ai_disclosure.ro": c.AIDisclosure["ro"],
	}
	for k, v := range req {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("missing %s", k)
		}
		if strings.Contains(v, openMark) || strings.Contains(v, closeMark) {
			return fmt.Errorf("%s still contains a ⟨placeholder⟩: %w", k, ErrPlaceholder)
		}
	}
	if !strings.HasPrefix(c.BaseURL, "https://") || strings.HasSuffix(c.BaseURL, "/") {
		return fmt.Errorf("base_url must start with https:// and have no trailing slash")
	}
	return nil
}

// URL scheme (spec §4.1). English at the root, Romanian under /ro/. Every URL ends with a slash.

func (c *Config) HomeURL(lang string) string {
	if lang == "ro" {
		return "/ro/"
	}
	return "/"
}

func (c *Config) IndexURL(lang string) string {
	if lang == "ro" {
		return "/ro/articole/"
	}
	return "/writing/"
}

func (c *Config) PieceURL(lang, slug string) string { return c.IndexURL(lang) + slug + "/" }

func (c *Config) PrivacyURL(lang string) string {
	if lang == "ro" {
		return "/ro/confidentialitate/"
	}
	return "/privacy/"
}

func (c *Config) ColophonURL() string { return "/colophon/" }

func (c *Config) FeedURL(lang string) string { // "" = all languages
	if lang == "" {
		return "/feed.xml"
	}
	return "/feed." + lang + ".xml"
}

func (c *Config) Abs(path string) string { return c.BaseURL + path }
