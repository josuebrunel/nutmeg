// Share widget on public pages (leaderboard, player profile) — always shares
// window.location.href read at click time, never a server-rendered URL, so
// it's correct for both slug and legacy UUID links without any coordination
// with the backend. Delegated on document so it works after htmx swaps.
document.addEventListener('click', function (evt) {
  var shareBtn = evt.target.closest('.share-btn')
  if (shareBtn) {
    var widget = shareBtn.closest('.share-widget')
    var title = widget.dataset.shareTitle || document.title
    if (navigator.share) {
      navigator.share({ title: title, url: window.location.href }).catch(function () {})
      return
    }
    var menu = widget.querySelector('.share-menu')
    document.querySelectorAll('.share-menu').forEach(function (m) {
      if (m !== menu) m.classList.add('hidden')
    })
    menu.classList.toggle('hidden')
    return
  }

  var copyBtn = evt.target.closest('.share-copy-link')
  if (copyBtn) {
    navigator.clipboard.writeText(window.location.href).then(function () {
      showToast('Link copied to clipboard', 'success')
    }).catch(function () {
      showToast('Could not copy link', 'error')
    })
    copyBtn.closest('.share-menu').classList.add('hidden')
    return
  }

  var xBtn = evt.target.closest('.share-x')
  if (xBtn) {
    var xTitle = xBtn.closest('.share-widget').dataset.shareTitle || document.title
    window.open('https://twitter.com/intent/tweet?url=' + encodeURIComponent(window.location.href) + '&text=' + encodeURIComponent(xTitle), '_blank', 'noopener')
    xBtn.closest('.share-menu').classList.add('hidden')
    return
  }

  var waBtn = evt.target.closest('.share-whatsapp')
  if (waBtn) {
    var waTitle = waBtn.closest('.share-widget').dataset.shareTitle || document.title
    window.open('https://wa.me/?text=' + encodeURIComponent(waTitle + ' ' + window.location.href), '_blank', 'noopener')
    waBtn.closest('.share-menu').classList.add('hidden')
    return
  }

  if (!evt.target.closest('.share-widget')) {
    document.querySelectorAll('.share-menu').forEach(function (m) { m.classList.add('hidden') })
  }
})
