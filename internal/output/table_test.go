package output

import "testing"

func TestMeetsFactorFilter(t *testing.T) {
	tests := []struct {
		name         string
		req, actual  int64
		metricsAvail bool
		threshold    int
		want         bool
	}{
		// Disabled filter
		{"threshold 0 always passes", 1000, 100, true, 0, true},
		{"threshold 0 passes even with no req", 0, 0, false, 0, true},

		// No req or no metrics → always excluded when filter is active
		{"no req excluded for positive threshold", 0, 100, true, 5, false},
		{"no req excluded for negative threshold", 0, 100, true, -1, false},
		{"no metrics excluded for positive threshold", 500, 50, false, 5, false},
		{"no metrics excluded for negative threshold", 500, 600, false, -1, false},

		// Positive threshold: req/actual >= threshold
		{"10x factor meets threshold 10", 1000, 100, true, 10, true},
		{"9x factor misses threshold 10", 900, 100, true, 10, false},
		{"50x factor meets threshold 10", 5000, 100, true, 10, true},
		{"actual 0 qualifies for any positive threshold", 500, 0, true, 3, true},

		// Negative threshold: bursting (actual > req)
		{"bursting pod matches negative threshold", 300, 500, true, -1, true},
		{"non-bursting pod excluded by negative threshold", 500, 300, true, -1, false},
		{"equal req and actual excluded by negative threshold", 500, 500, true, -1, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := meetsFactorFilter(tc.req, tc.actual, tc.metricsAvail, tc.threshold)
			if got != tc.want {
				t.Errorf("meetsFactorFilter(req=%d, actual=%d, metrics=%v, threshold=%d) = %v, want %v",
					tc.req, tc.actual, tc.metricsAvail, tc.threshold, got, tc.want)
			}
		})
	}
}
