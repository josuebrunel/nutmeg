// Powers the "Log Match" form: tap +/- to tally goals, and keep the final
// score in sync with those tallies unless the user has typed a score by hand
// (e.g. to account for an own goal). Listeners are delegated on `document` so
// they keep working after htmx swaps the form in/out without re-init.

document.addEventListener('click', function (evt) {
  var incBtn = evt.target.closest('.goal-inc')
  var decBtn = evt.target.closest('.goal-dec')
  if (!incBtn && !decBtn) return

  var stepper = evt.target.closest('.goal-stepper')
  if (!stepper) return

  var input = stepper.querySelector('.goal-input')
  var display = stepper.querySelector('.goal-count')
  var value = parseInt(input.value, 10) || 0
  value = incBtn ? value + 1 : Math.max(0, value - 1)

  input.value = value
  display.textContent = value
  recomputeScore(stepper.closest('form'))
})

document.addEventListener('click', function (evt) {
  var incBtn = evt.target.closest('.assist-inc')
  var decBtn = evt.target.closest('.assist-dec')
  if (!incBtn && !decBtn) return

  var stepper = evt.target.closest('.assist-stepper')
  if (!stepper) return

  var input = stepper.querySelector('.assist-input')
  var display = stepper.querySelector('.assist-count')
  var value = parseInt(input.value, 10) || 0
  value = incBtn ? value + 1 : Math.max(0, value - 1)

  input.value = value
  display.textContent = value
})

document.addEventListener('change', function (evt) {
  if (evt.target.matches('.team-radio')) {
    recomputeScore(evt.target.closest('form'))
  }
  if (evt.target.matches('.score-input')) {
    evt.target.dataset.manual = 'true'
  }
})

function recomputeScore(form) {
  if (!form) return

  var totals = { a: 0, b: 0 }
  form.querySelectorAll('.match-player-row').forEach(function (row) {
    var checked = row.querySelector('.team-radio:checked')
    var team = checked ? checked.value : ''
    if (team !== 'a' && team !== 'b') return
    var goalInput = row.querySelector('.goal-input')
    totals[team] += goalInput ? parseInt(goalInput.value, 10) || 0 : 0
  })

  var scoreA = form.querySelector('.score-input[data-team="a"]')
  var scoreB = form.querySelector('.score-input[data-team="b"]')
  if (scoreA && scoreA.dataset.manual !== 'true') scoreA.value = totals.a
  if (scoreB && scoreB.dataset.manual !== 'true') scoreB.value = totals.b
}
