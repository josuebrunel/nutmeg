document.addEventListener('DOMContentLoaded', function () {
  // Mobile menu toggle
  const menuBtn = document.getElementById('mobile-menu-btn')
  const mobileMenu = document.getElementById('mobile-menu')
  if (menuBtn && mobileMenu) {
    menuBtn.addEventListener('click', function () {
      mobileMenu.classList.toggle('hidden')
    })
  }

  // HTMX handlers
  document.body.addEventListener('htmx:afterSwap', function (evt) {
    // Open match modal when content is loaded into it
    if (evt.target.id === 'match-modal-container') {
      document.getElementById('match-modal').showModal()
    }
    // Open manage group modal
    if (evt.target.id === 'manage-group-container') {
      document.getElementById('manage-group-modal').showModal()
    }
  })

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
      // Also check for flash cookie
      var flashMsg = evt.detail.xhr.getResponseHeader('X-Flash-Message')
      var flashType = evt.detail.xhr.getResponseHeader('X-Flash-Type')
      if (flashMsg) {
        showToast(flashMsg, flashType || 'success')
      }
    }
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
