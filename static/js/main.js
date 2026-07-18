function toggleSidebar() {
  var sidebar = document.getElementById('app-sidebar');
  var overlay = document.getElementById('sidebar-overlay');
  sidebar.classList.toggle('-translate-x-full');
  if (overlay) {
    overlay.classList.toggle('hidden');
  }
}

function closeModal() {
  var modal = document.getElementById('match-modal');
  if (modal) modal.close();
}

function showToast(msg, icon) {
  icon = icon || '✅';
  var container = document.getElementById('toast-container');
  var toast = document.createElement('div');
  toast.className = 'toast-item';
  toast.innerHTML = icon + ' ' + msg;
  container.appendChild(toast);
  setTimeout(function () { toast.remove(); }, 3000);
}

document.addEventListener('DOMContentLoaded', function () {
  var sidebar = document.getElementById('app-sidebar');
  var overlay = document.getElementById('sidebar-overlay');
  if (overlay) {
    overlay.addEventListener('click', function () {
      sidebar.classList.add('-translate-x-full');
      overlay.classList.add('hidden');
    });
  }
  document.body.addEventListener('htmx:afterSwap', function (evt) {
    if (evt.detail.target && evt.detail.target.id === 'match-modal-container') {
      var modal = document.getElementById('match-modal');
      if (modal) modal.showModal();
    }
    var flash = evt.detail?.elt?.querySelector?.('[data-flash-toast]');
    if (flash) {
      showToast(flash.textContent, flash.dataset.flashIcon || '✅');
      flash.remove();
    }
  });
  document.body.addEventListener('htmx:beforeSwap', function (evt) {
    if (evt.detail.isError) {
      showToast('An error occurred', '❌');
    }
  });
});
