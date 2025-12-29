// Toast Notification System
(function() {
  'use strict';

  const TOAST_DURATION = 5000;
  const container = document.getElementById('toast-container');

  function createToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast toast--${type}`;
    toast.setAttribute('role', 'alert');
    toast.innerHTML = `
      <span class="toast__icon">${getIcon(type)}</span>
      <span class="toast__message">${escapeHtml(message)}</span>
      <button type="button" class="toast__close" aria-label="Dismiss">&times;</button>
      <div class="toast__progress"></div>
    `;

    // Close button handler
    toast.querySelector('.toast__close').onclick = () => removeToast(toast);

    container.appendChild(toast);

    // Trigger animation
    requestAnimationFrame(() => toast.classList.add('toast--visible'));

    // Auto dismiss
    setTimeout(() => removeToast(toast), TOAST_DURATION);
  }

  function removeToast(toast) {
    toast.classList.remove('toast--visible');
    toast.classList.add('toast--hiding');
    setTimeout(() => toast.remove(), 200);
  }

  function getIcon(type) {
    switch (type) {
      case 'success': return '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>';
      case 'error': return '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>';
      case 'warning': return '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>';
      default: return '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>';
    }
  }

  function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  // Listen for HTMX events with showToast trigger
  document.body.addEventListener('showToast', function(evt) {
    const detail = evt.detail || {};
    createToast(detail.message || 'Notification', detail.type || 'info');
  });

  // Also handle HX-Trigger header format
  document.body.addEventListener('htmx:afterRequest', function(evt) {
    const trigger = evt.detail.xhr.getResponseHeader('HX-Trigger');
    if (trigger) {
      try {
        const parsed = JSON.parse(trigger);
        if (parsed.showToast) {
          const t = parsed.showToast;
          createToast(t.message || 'Notification', t.type || 'info');
        }
      } catch (e) {
        // Not JSON, might be simple event name
        if (trigger === 'showToast') {
          createToast('Action completed', 'success');
        }
      }
    }
  });

  // Expose globally for manual triggers
  window.showToast = createToast;
})();
