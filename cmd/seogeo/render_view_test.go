package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderViewFlag: --view selects the version the HTML opens on (and
// therefore the one printed to PDF).
func TestRenderViewFlag(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join("..", "..", "examples", "demo-en.json")
	for view, want := range map[string]string{"": "essential", "essential": "essential", "advanced": "advanced", "avance": "advanced"} {
		out := filepath.Join(dir, "r-"+view+".html")
		args := []string{"-i", in, "-o", out, "--no-favicon"}
		if view != "" {
			args = append(args, "--view", view)
		}
		if err := cmdRender(args); err != nil {
			t.Fatalf("--view %q: %v", view, err)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		i := strings.Index(s, `id="view-`+want+`"`)
		if i < 0 || !strings.HasSuffix(s[i:i+strings.Index(s[i:], ">")], " checked") {
			t.Errorf("--view %q: the %s version should be checked", view, want)
		}
	}
	if err := cmdRender([]string{"-i", in, "-o", filepath.Join(dir, "x.html"), "--no-favicon", "--view", "medium"}); err == nil {
		t.Error("an unknown --view should fail")
	}
}
