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
