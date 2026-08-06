package components

// Position codes, mirroring model.PositionGK/D/M/A — the short values stored
// on a roster member and rendered in position badges and pickers.
const (
	PositionGK = "GK"
	PositionD  = "D"
	PositionM  = "M"
	PositionA  = "A"
)

// positionOption pairs a stored short position code (see
// model.PositionGK/D/M/A) with the label shown in the picker.
type positionOption struct {
	Value string
	Label string
}

// PlayerPositions is the fixed, ordered set of position choices — goalkeeper
// through attacker, standard lineup order. Used by the roster quick-add
// form's dropdown and the roster row's inline tap-to-set picker.
var PlayerPositions = []positionOption{
	{PositionGK, "GK (Goalkeeper)"},
	{PositionD, "D (Defender)"},
	{PositionM, "M (Midfielder)"},
	{PositionA, "A (Attacker)"},
}

// PositionBadge returns the color pill classes for a roster/leaderboard
// position badge — the soft tint + readable text color for that position.
// Callers add their own base sizing classes (text size, padding, radius).
func PositionBadge(code string) string {
	_, tint := positionColors(code)
	return tint
}

// PositionSolid returns the solid accent classes for a position — a saturated
// fill intended for dark backgrounds (e.g. a player badge on the green
// stadium-gradient header), where the light tint would be illegible.
func PositionSolid(code string) string {
	solid, _ := positionColors(code)
	return solid
}

// PositionPillClass returns the classes for an inline position picker pill:
// the active/selected pill renders as a solid accent for its position, while
// the other three show the position's color as a faint, tappable tint.
func PositionPillClass(code, active string) string {
	if code == active {
		solid, _ := positionColors(code)
		return "px-1.5 py-1 " + solid + " transition-colors"
	}
	_, tint := positionColors(code)
	return "px-1.5 py-1 " + tint + " opacity-50 hover:opacity-100 transition-opacity"
}

// positionColors maps each position code to a color pair for its pills:
// solid (a high-contrast filled state for the active/pill-accent) and
// tint (a soft tinted background + readable text for a passive badge).
// Uses the same distinguishable hues as the football-position color code:
// yellow for keeper, blue for defenders, green for midfield, red for attack.
func positionColors(code string) (solid, tint string) {
	switch code {
	case PositionGK:
		return "bg-yellow-400 text-yellow-950", "bg-yellow-100 text-yellow-900"
	case PositionD:
		return "bg-blue-500 text-white", "bg-blue-100 text-blue-900"
	case PositionM:
		return "bg-green-500 text-white", "bg-green-100 text-green-900"
	case PositionA:
		return "bg-red-500 text-white", "bg-red-100 text-red-900"
	default:
		return "bg-turf text-chalk", "bg-turf/10 text-turf"
	}
}
