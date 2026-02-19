package analysis

import "github.com/jedib0t/go-pretty/v6/text"

// Verdict represents the health verdict for a resource dimension.
type Verdict struct {
	Label string
	Color text.Color
}

var (
	VerdictMassivelyOverRequested = Verdict{"Massively over-requested", text.FgRed}
	VerdictOverRequested          = Verdict{"Over-requested", text.FgYellow}
	VerdictBursting               = Verdict{"Bursting", text.FgMagenta}
	VerdictOK                     = Verdict{"OK", text.FgGreen}
)

// ResourceVerdict returns the verdict given requested% and actual% usage.
func ResourceVerdict(requestedPct, actualPct float64) Verdict {
	diff := requestedPct - actualPct
	switch {
	case diff > 50:
		return VerdictMassivelyOverRequested
	case diff > 20:
		return VerdictOverRequested
	case actualPct > requestedPct:
		return VerdictBursting
	default:
		return VerdictOK
	}
}

// FactorColors returns the display colors for a CPU over-request factor.
// req and actual are in millicores.
func FactorColors(req, actual int64) text.Colors {
	if req == 0 || actual == 0 {
		return text.Colors{text.Faint}
	}
	factor := req / actual
	switch {
	case factor >= 50:
		return text.Colors{text.Bold, text.FgRed}
	case factor >= 10:
		return text.Colors{text.FgRed}
	case factor >= 3:
		return text.Colors{text.FgYellow}
	default:
		return text.Colors{text.FgGreen}
	}
}
