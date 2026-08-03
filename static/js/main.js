document.addEventListener('DOMContentLoaded', function () {
  // Mobile menu toggle
  const menuBtn = document.getElementById('mobile-menu-btn')
  const mobileMenu = document.getElementById('mobile-menu')
  if (menuBtn && mobileMenu) {
    menuBtn.addEventListener('click', function () {
      mobileMenu.classList.toggle('hidden')
    })
  }

  // Flash toasts via HTMX
  document.body.addEventListener('htmx:beforeSwap', function (evt) {
    if (evt.detail.xhr) {
      var hxTrigger = evt.detail.xhr.getResponseHeader('HX-Trigger')
      if (hxTrigger) {
        try {
          var data = JSON.parse(hxTrigger)
          if (data.showToast) {
            showToast(data.showToast.message, data.showToast.type)
          }
        } catch (e) {}
      }
      var flashMsg = evt.detail.xhr.getResponseHeader('X-Flash-Message')
      var flashType = evt.detail.xhr.getResponseHeader('X-Flash-Type')
      if (flashMsg) {
        showToast(flashMsg, flashType || 'success')
      }
    }
  })
})

// "Show all (N more)" controls on long lists (Leaderboard, Roster, Recent
// Matches) — delegated on document so it keeps working after htmx swaps.
document.addEventListener('click', function (evt) {
  var btn = evt.target.closest('.show-more-btn')
  if (!btn) return
  document.querySelectorAll(btn.dataset.target).forEach(function (el) {
    el.classList.remove('hidden')
  })
  btn.parentElement.remove()
})

// Score-ranking explainer — tap/click toggle (not a hover tooltip, so it
// works on touch devices) revealing the .score-info-text paragraph inside
// the same .score-info-group wrapper as the button. Used by both the
// leaderboard's ranking explainer and the player profile's Pts/Game tile,
// which have different internal layouts, hence the closest-ancestor
// lookup rather than an assumed sibling order. Delegated on document so
// it keeps working after htmx swaps.
document.addEventListener('click', function (evt) {
  var btn = evt.target.closest('.score-info-btn')
  if (!btn) return
  var group = btn.closest('.score-info-group')
  var text = group && group.querySelector('.score-info-text')
  if (!text) return
  var hidden = text.classList.toggle('hidden')
  btn.setAttribute('aria-expanded', hidden ? 'false' : 'true')
})

// Player-name search — leaderboard (only in the expanded full-width view)
// and roster (same "Show all" gating, see RosterColumn/visibleMembers)
// share this one listener since both just filter [data-player-name] rows
// by substring as the user types. Delegated on document so it keeps
// working after htmx swaps.
document.addEventListener('input', function (evt) {
  if (!evt.target.matches('.leaderboard-search, .roster-search')) return
  var query = evt.target.value.trim().toLowerCase()
  document.querySelectorAll('[data-player-name]').forEach(function (row) {
    row.classList.toggle('hidden', query !== '' && !row.dataset.playerName.includes(query))
  })
})

// Public group page's Leaderboard / Recent Matches tab toggle — both
// panels are already rendered on page load (no extra request needed,
// unlike the private page's htmx-swapped tabs), so this just shows/hides
// them and updates the active button style client-side. Delegated on
// document so it keeps working after htmx swaps (e.g. the match-article
// overlay).
document.addEventListener('click', function (evt) {
  var btn = evt.target.closest('.public-tab-btn')
  if (!btn) return
  var target = document.querySelector(btn.dataset.target)
  if (!target) return
  btn.parentElement.querySelectorAll('.public-tab-btn').forEach(function (b) {
    b.classList.remove('border-turf', 'text-turf')
    b.classList.add('border-transparent', 'text-ink/60')
  })
  btn.classList.remove('border-transparent', 'text-ink/60')
  btn.classList.add('border-turf', 'text-turf')
  document.querySelectorAll('.public-tab-panel').forEach(function (panel) {
    panel.classList.toggle('hidden', panel !== target)
  })
})

function showToast(message, type) {
  var container = document.getElementById('toast-container')
  if (!container) return
  var toast = document.createElement('div')
  toast.className = 'toast-item' + (type === 'error' ? ' error' : '')
  toast.textContent = message
  container.appendChild(toast)
  setTimeout(function () {
    if (toast.parentNode) toast.remove()
  }, 3200)
}
