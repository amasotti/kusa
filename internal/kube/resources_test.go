package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestMillicoresFromQuantity(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"500m", 500},
		{"1", 1000},   // 1 core
		{"2.5", 2500}, // 2.5 cores
		{"100m", 100},
		{"0", 0},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			q := resource.MustParse(tc.input)
			if got := MillicoresFromQuantity(q); got != tc.want {
				t.Errorf("MillicoresFromQuantity(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestMiBFromQuantity(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"512Mi", 512},
		{"1Gi", 1024},
		{"1536Mi", 1536}, // 1.5 Gi
		{"256Mi", 256},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			q := resource.MustParse(tc.input)
			if got := MiBFromQuantity(q); got != tc.want {
				t.Errorf("MiBFromQuantity(%q) = %f, want %f", tc.input, got, tc.want)
			}
		})
	}
}

func TestFormatMem(t *testing.T) {
	tests := []struct {
		mib  float64
		want string
	}{
		{0, "0Mi"},
		{512, "512Mi"},
		{1023, "1023Mi"},
		{1024, "1Gi"}, // exact GiB, no decimal
		{2048, "2Gi"},
		{1536, "1.5Gi"},
		{1024 + 512, "1.5Gi"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := FormatMem(tc.mib); got != tc.want {
				t.Errorf("FormatMem(%g) = %q, want %q", tc.mib, got, tc.want)
			}
		})
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		millicores int64
		want       string
	}{
		{0, "0"},
		{250, "250m"},
		{999, "999m"},
		{1000, "1"}, // exact core, no decimal
		{2000, "2"},
		{1500, "1.50"}, // fractional cores
		{2500, "2.50"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := FormatCPU(tc.millicores); got != tc.want {
				t.Errorf("FormatCPU(%d) = %q, want %q", tc.millicores, got, tc.want)
			}
		})
	}
}

func TestFormatFactor(t *testing.T) {
	tests := []struct {
		req, actual int64
		want        string
	}{
		{0, 100, "no req"}, // no request set
		{100, 0, "N/A"},    // pod used nothing (avoid divide-by-zero)
		{100, 10, "10x"},
		{500, 5, "100x"},
		{50, 100, "0x"}, // actual > req → factor rounds to 0
		{100, 100, "1x"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := FormatFactor(tc.req, tc.actual); got != tc.want {
				t.Errorf("FormatFactor(%d, %d) = %q, want %q", tc.req, tc.actual, got, tc.want)
			}
		})
	}
}
