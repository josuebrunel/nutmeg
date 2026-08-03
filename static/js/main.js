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

// Leaderboard ranking explainer — tap/click toggle (not a hover tooltip, so
// it works on touch devices) revealing the score-info-text paragraph right
// after the button's row. Delegated on document so it keeps working after
// htmx swaps.
document.addEventListener('click', function (evt) {
  var btn = evt.target.closest('.score-info-btn')
  if (!btn) return
  var text = btn.parentElement.nextElementSibling
  if (!text) return
  var hidden = text.classList.toggle('hidden')
  btn.setAttribute('aria-expanded', hidden ? 'false' : 'true')
})

// Leaderboard search — only rendered in the expanded (full-width) leaderboard
// view; filters visible rows by player name as the user types. Delegated on
// document so it keeps working after htmx swaps.
document.addEventListener('input', function (evt) {
  if (!evt.target.matches('.leaderboard-search')) return
  var query = evt.target.value.trim().toLowerCase()
  document.querySelectorAll('[data-player-name]').forEach(function (row) {
    row.classList.toggle('hidden', query !== '' && !row.dataset.playerName.includes(query))
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
