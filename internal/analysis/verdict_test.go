package analysis

import (
	"testing"

	"github.com/jedib0t/go-pretty/v6/text"
)

func TestResourceVerdict(t *testing.T) {
	tests := []struct {
		name         string
		requestedPct float64
		actualPct    float64
		want         Verdict
	}{
		// Massively over-requested: diff > 50
		{"diff of 60 is massive", 80, 20, VerdictMassivelyOverRequested},
		{"diff of 51 is massive", 71, 20, VerdictMassivelyOverRequested},

		// Over-requested: diff > 20 (but ≤ 50)
		{"diff of 25 is over-requested", 50, 25, VerdictOverRequested},
		{"diff of 50 exactly is over-requested not massive", 70, 20, VerdictOverRequested},
		{"diff of 21 is over-requested", 41, 20, VerdictOverRequested},

		// Bursting: actual > requested
		{"actual exceeds requested", 20, 35, VerdictBursting},
		{"actual just above requested", 30, 31, VerdictBursting},

		// OK: small diff, actual not exceeding requested
		{"diff of 20 exactly is OK", 40, 20, VerdictOK},
		{"diff of 5 is OK", 30, 25, VerdictOK},
		{"equal is OK", 50, 50, VerdictOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResourceVerdict(tc.requestedPct, tc.actualPct)
			if got != tc.want {
				t.Errorf("ResourceVerdict(%.0f, %.0f) = %q, want %q",
					tc.requestedPct, tc.actualPct, got.Label, tc.want.Label)
			}
		})
	}
}

func TestFactorColors(t *testing.T) {
	tests := []struct {
		name        string
		req, actual int64
		wantFirst   text.Color // first (or only) element of the returned Colors slice
		wantLen     int
	}{
		{"no request → dim", 0, 100, text.Faint, 1},
		{"zero actual → dim", 200, 0, text.Faint, 1},
		{"both zero → dim", 0, 0, text.Faint, 1},

		// factor = req/actual (integer division)
		{"factor 50 → bold red (≥50)", 5000, 100, text.Bold, 2}, // factor=50
		{"factor 51 → bold red (≥50)", 5100, 100, text.Bold, 2}, // factor=51
		{"factor 49 → red (≥10)", 4900, 100, text.FgRed, 1},     // factor=49
		{"factor 10 → red (≥10)", 1000, 100, text.FgRed, 1},
		{"factor 3 → yellow (≥3)", 300, 100, text.FgYellow, 1},
		{"factor 2 → green (<3)", 200, 100, text.FgGreen, 1},
		{"factor 1 → green (<3)", 100, 100, text.FgGreen, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FactorColors(tc.req, tc.actual)
			if len(got) != tc.wantLen {
				t.Errorf("FactorColors(%d, %d): got %d colors, want %d", tc.req, tc.actual, len(got), tc.wantLen)
				return
			}
			if got[0] != tc.wantFirst {
				t.Errorf("FactorColors(%d, %d): first color = %v, want %v", tc.req, tc.actual, got[0], tc.wantFirst)
			}
		})
	}
}
