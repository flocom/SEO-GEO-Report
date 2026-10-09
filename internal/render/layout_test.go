package render

import (
	"fmt"
	"strings"
	"testing"
)

// TestBalancedGrid checks that, for 1 to 9 items and every preset and
// breakpoint, rows are balanced: no row differs from another by more than
// one item, no lone item when it can be avoided, and spans fill each row.
func TestBalancedGrid(t *testing.T) {
	for name, p := range gridPresets {
		for n := 1; n <= 9; n++ {
			g := balFor(name, n)
			for bp := 0; bp < 4; bp++ {
				mx := p[bp]
				rows := balancedRows(n, mx)
				lo, hi := n, 0
				sum := 0
				for _, r := range rows {
					lo, hi = min(lo, r), max(hi, r)
					sum += r
					if r > mx {
						t.Errorf("%s n=%d bp=%d: row of %d exceeds %d columns", name, n, bp, r, mx)
					}
				}
				if sum != n || hi-lo > 1 {
					t.Errorf("%s n=%d bp=%d: unbalanced rows %v", name, n, bp, rows)
				}
				if len(rows) > 1 && rows[len(rows)-1] == 1 && rows[0] > 1 {
					t.Errorf("%s n=%d bp=%d: lone item on the last row %v", name, n, bp, rows)
				}
				// Every row fills the grid exactly.
				tracks, spans := g.tracks[bp], g.spans[bp]
				i := 0
				for _, r := range rows {
					w := 0
					for k := 0; k < r; k++ {
						w += spans[i]
						i++
					}
					if w != tracks {
						t.Errorf("%s n=%d bp=%d: row spans %d of %d tracks", name, n, bp, w, tracks)
					}
				}
			}
		}
	}
}

func TestBalancedRowsExamples(t *testing.T) {
	cases := []struct {
		n, max int
		want   string
	}{
		{6, 6, "[6]"}, {5, 6, "[5]"}, {6, 3, "[3 3]"}, {5, 3, "[3 2]"}, {4, 3, "[2 2]"},
		{7, 3, "[3 2 2]"}, {9, 5, "[5 4]"}, {3, 2, "[1 1 1]"}, {1, 4, "[1]"}, {0, 3, "[]"},
	}
	for _, c := range cases {
		got := fmt.Sprint(balancedRows(c.n, c.max))
		if got != c.want {
			t.Errorf("balancedRows(%d,%d) = %s, want %s", c.n, c.max, got, c.want)
		}
	}
	// CSS output.
	g := newBalGrid(5, 6, 3, 1, 3)
	if s := string(g.Style()); s != "--bd:5;--bt:6;--bm:1;--bp:6;" {
		t.Errorf("Style = %q", s)
	}
	if s := string(g.At(4)); s != "--sd:1;--st:3;--sm:1;--sp:3;" {
		t.Errorf("At(4) = %q", s)
	}
}

// TestBalanceHalfWidth checks that runs of half-width cards never leave a
// card alone on its row: an odd run ends with a full-width card.
func TestBalanceHalfWidth(t *testing.T) {
	for n := 1; n <= 9; n++ {
		blocks := []block{{Span: 12}}
		for i := 0; i < n; i++ {
			blocks = append(blocks, block{Span: 6})
		}
		blocks = append(blocks, block{Span: 12})
		balance(blocks)
		row := 0
		for i, b := range blocks {
			row += b.Span
			if row > 12 {
				t.Fatalf("n=%d: block %d overflows its row", n, i)
			}
			if row == 12 {
				row = 0
			}
		}
		if row != 0 {
			t.Errorf("n=%d: last row is incomplete (%d/12)", n, row)
		}
	}
}

// TestRenderedGridsBalanced renders the demo report and checks every
// balanced grid declares its tracks and every item its spans.
func TestRenderedGridsBalanced(t *testing.T) {
	out, err := HTML(richReport("fr"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, `class="kpi-grid bal n-6 big" style="--bd:6;--bt:3;--bm:2;--bp:3;"`) {
		t.Error("Part 1 key indicators should sit on one row on desktop (6 tracks)")
	}
	for _, cls := range []string{`class="kpi-grid bal`, `class="mini-stats bal"`, `class="cover-meta bal"`} {
		if !strings.Contains(s, cls) {
			t.Errorf("missing balanced grid %s", cls)
		}
	}
	if strings.Count(s, `bal" style="--bd:`)+strings.Count(s, `big" style="--bd:`)+strings.Count(s, `compact" style="--bd:`) == 0 {
		t.Error("balanced grids carry no layout")
	}
}
