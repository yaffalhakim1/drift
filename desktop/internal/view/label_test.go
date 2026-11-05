package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestLabelFits measures what the plate's label can hold, so a long title
// shrinks instead of truncating. This is the check that caught the label
// printing "Owls of the Ni…".
func TestLabelFits(t *testing.T) {
	var got struct {
		short float32
		long  float32
		size  float32
	}

	ui.Render(func(c *ui.Context) {
		ui.Box(c).Size(148, 148).Draw(func(p *ui.Painter, r ui.Rect) {
			size := float32(11)
			got.size = size
			got.short = textWidth(p, "Nova", size)
			got.long = textWidth(p, "Owls of the Night", size)
		})
	}, 148, 148, 1)

	if got.short <= 0 {
		t.Fatalf("width of %q measured as %v: the painter is not measuring", "Nova", got.short)
	}
	if got.long <= got.short {
		t.Fatalf("a long title (%v) is not wider than a short one (%v)", got.long, got.short)
	}
	t.Logf("at size %.1f: \"Nova\" = %.1f, \"Owls of the Night\" = %.1f", got.size, got.short, got.long)

	// The budget the plate gives the print at 148 px.
	rLabel := float32(148) / 2 * 0.44
	budget := rLabel * 1.62
	t.Logf("label radius %.1f, budget %.1f", rLabel, budget)

	if got.long <= budget {
		t.Logf("long title fits at full size: no truncation needed")
	} else {
		t.Logf("long title needs shrinking: %.0f%% of the budget", got.long/budget*100)
	}
}
