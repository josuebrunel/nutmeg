package players

// ChartData is the data passed to static/js/player-chart.js via
// templ.JSONScript to render the profile page's stats charts — sourced
// directly from repository.PlayerStats/PlayerMatchResult, no invented
// numbers.
type ChartData struct {
	Wins   int      `json:"wins"`
	Draws  int      `json:"draws"`
	Losses int      `json:"losses"`
	Labels []string `json:"labels"`
	Goals  []int    `json:"goals"`
}
