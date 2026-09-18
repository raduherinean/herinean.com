package content

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Which AI projects are worth funding in 2027": "which-ai-projects-are-worth-funding-in-2027",
		"Ce ar trebui să întrebe un board despre AI":  "ce-ar-trebui-sa-intrebe-un-board-despre-ai",
		"Ș ț Ă Â Î — și „ghilimele":                   "s-t-a-a-i-si-ghilimele",
		"  double  spaces -- and_underscores ":        "double-spaces-and-underscores",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
	for _, bad := range []string{"", "-a", "a-", "a--b", "Abc", "ș", "a b"} {
		if ValidSlug(bad) {
			t.Errorf("ValidSlug(%q) should be false", bad)
		}
	}
	if !ValidSlug("ai-funding-2027") {
		t.Error("ValidSlug(ai-funding-2027) should be true")
	}
}
