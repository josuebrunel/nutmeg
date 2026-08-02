// Player profile stats charts (Chart.js) — no-op on any page that doesn't
// have the chart data payload (i.e. every page except a profile with at
// least one match played). Data comes straight from repository.PlayerStats
// / PlayerMatchResult via the server — nothing computed client-side.
document.addEventListener('DOMContentLoaded', function () {
  var payload = document.getElementById('player-chart-data')
  if (!payload || typeof Chart === 'undefined') return

  var data
  try {
    data = JSON.parse(payload.textContent)
  } catch (e) {
    return
  }

  var recordCanvas = document.getElementById('player-chart-record')
  if (recordCanvas) {
    new Chart(recordCanvas, {
      type: 'doughnut',
      data: {
        labels: ['Wins', 'Draws', 'Losses'],
        datasets: [{
          data: [data.wins, data.draws, data.losses],
          backgroundColor: ['#1ED760', '#9CA3AF', '#FF4500'],
        }],
      },
      options: { plugins: { legend: { position: 'bottom' } } },
    })
  }

  var goalsCanvas = document.getElementById('player-chart-goals')
  if (goalsCanvas && data.labels && data.labels.length) {
    new Chart(goalsCanvas, {
      type: 'bar',
      data: {
        labels: data.labels,
        datasets: [{
          label: 'Goals',
          data: data.goals,
          backgroundColor: '#FFD700',
        }],
      },
      options: {
        scales: { y: { beginAtZero: true, ticks: { stepSize: 1 } } },
        plugins: { legend: { display: false } },
      },
    })
  }
})
