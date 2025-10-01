(function () {
  document.addEventListener('htmx:afterRequest', function (event) {
    if (!(event.target instanceof HTMLFormElement)) return;
    if (event.target.id !== 'register-form') return;

    var messageEl = document.getElementById('register-message');
    if (!messageEl) return;

    messageEl.classList.remove('success');

    var xhr = event.detail ? event.detail.xhr : null;
    var responseText = xhr && typeof xhr.response === 'string' ? xhr.response : '';
    var displayText = '';

    try {
      var payload = responseText ? JSON.parse(responseText) : null;
      if (payload) {
        var key = event.detail.successful ? 'message' : 'error';
        displayText = payload[key] || '';
      }
    } catch (_err) {
      displayText = responseText;
    }

    if (event.detail.successful) {
      messageEl.classList.add('success');
    }

    messageEl.textContent = displayText || (event.detail.successful ? 'Registration complete!' : 'Registration failed. Please try again.');
  });
})();
