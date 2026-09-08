(function () {
  'use strict';

  let TOAST_DURATION_MS = 5000;

  function showToast(message) {
    let root = document.getElementById('toast-root');
    if (!root) return;
    let toast = document.createElement('div');
    toast.className = 'app-toast';
    toast.textContent = message;
    root.appendChild(toast);
    setTimeout(function () {
      if (toast.parentNode) {
        toast.remove();
      }
    }, TOAST_DURATION_MS);
  }

  document.addEventListener('htmx:responseError', function (evt) {
    let xhr = evt.detail?.xhr;
    let status = xhr ? xhr.status : 0;
    if (status < 400 || status >= 600) {
      return;
    }
    let text = ((xhr?.responseText) || '').trim();
    if (!text) {
      text = 'Request failed (' + status + ')';
    }
    showToast(text);
  });
})();