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

  function showMarkdownTab(tab) {
    let editor = tab.closest('.markdown-editor');
    if (!editor) return;

    let name = tab.getAttribute('data-markdown-tab');

    editor.querySelectorAll('[data-markdown-tab]').forEach(function (candidate) {
      let active = candidate === tab;
      candidate.classList.toggle('active', active);
      candidate.setAttribute('aria-selected', active ? 'true' : 'false');
    });

    editor.querySelectorAll('[data-markdown-pane]').forEach(function (pane) {
      pane.classList.toggle('active', pane.getAttribute('data-markdown-pane') === name);
    });

    if (name === 'preview') {
      let preview = editor.querySelector('[data-markdown-pane="preview"]');
      if (preview) {
        preview.dispatchEvent(new Event('markdown-preview-shown'));
      }
    } else {
      let source = editor.querySelector('textarea');
      if (source) source.focus();
    }
  }

  document.addEventListener('click', function (evt) {
    let tab = evt.target.closest('[data-markdown-tab]');
    if (!tab) return;
    evt.preventDefault();
    showMarkdownTab(tab);
  });

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