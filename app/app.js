(function () {
  function removeLoadingClass() {
    document.body.classList.remove('is-loading');
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', removeLoadingClass, { once: true });
  } else {
    removeLoadingClass();
  }

  function ensureHTMX() {
    if (document.documentElement.dataset.htmxLoaded) return;
    document.documentElement.dataset.htmxLoaded = 'true';
    var script = document.createElement('script');
    script.src = 'https://cdn.jsdelivr.net/npm/htmx.org@2.0.7/dist/htmx.min.js';
    script.integrity = 'sha384-ZBXiYtYQ6hJ2Y0ZNoYuI+Nq5MqWBr+chMrS/RkXpNzQCApHEhOt2aY8EJgqwHLkJ';
    script.crossOrigin = 'anonymous';
    document.head.appendChild(script);
  }

  if ('requestIdleCallback' in window) {
    requestIdleCallback(ensureHTMX);
  } else {
    window.addEventListener('load', ensureHTMX, { once: true });
  }
})();
