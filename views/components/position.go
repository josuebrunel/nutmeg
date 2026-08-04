package components

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
	{"GK", "GK (Goalkeeper)"},
	{"D", "D (Defender)"},
	{"M", "M (Midfielder)"},
	{"A", "A (Attacker)"},
}
