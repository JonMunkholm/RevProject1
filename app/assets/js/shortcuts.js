// Keyboard Shortcuts System
(function() {
  'use strict';

  // Key handler configuration
  const shortcuts = {
    // Cmd/Ctrl + Enter: Submit form
    'meta+Enter': submitCurrentForm,
    'ctrl+Enter': submitCurrentForm,

    // Escape: Close sidebar, dismiss modals
    'Escape': handleEscape,

    // Slash: Focus chat input
    '/': focusChatInput,

    // N: New conversation (when not in input)
    'n': newConversation,
  };

  function submitCurrentForm() {
    // Find the focused textarea and submit its form
    const focused = document.activeElement;
    if (focused && focused.tagName === 'TEXTAREA') {
      const form = focused.closest('form');
      if (form) {
        htmx.trigger(form, 'submit');
        return true;
      }
    }
    return false;
  }

  function handleEscape() {
    // Close mobile sidebar
    if (document.body.classList.contains('sidebar-open')) {
      document.body.classList.remove('sidebar-open');
      return true;
    }

    // Blur focused input
    const focused = document.activeElement;
    if (focused && (focused.tagName === 'INPUT' || focused.tagName === 'TEXTAREA')) {
      focused.blur();
      return true;
    }

    return false;
  }

  function focusChatInput(event) {
    // Only trigger when not already in an input
    if (isInInput()) return false;

    const chatInput = document.querySelector('.chat-composer__textarea');
    if (chatInput) {
      event.preventDefault();
      chatInput.focus();
      return true;
    }
    return false;
  }

  function newConversation(event) {
    // Only trigger when not in an input
    if (isInInput()) return false;

    const newChatBtn = document.querySelector('[hx-post*="/conversations"]');
    if (newChatBtn) {
      event.preventDefault();
      newChatBtn.click();
      return true;
    }
    return false;
  }

  function isInInput() {
    const tag = document.activeElement?.tagName;
    return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT';
  }

  function getShortcutKey(event) {
    const parts = [];
    if (event.metaKey) parts.push('meta');
    if (event.ctrlKey) parts.push('ctrl');
    if (event.altKey) parts.push('alt');
    if (event.shiftKey) parts.push('shift');
    parts.push(event.key);
    return parts.join('+');
  }

  // Global keyboard listener
  document.addEventListener('keydown', function(event) {
    const key = getShortcutKey(event);
    const handler = shortcuts[key];

    if (handler && typeof handler === 'function') {
      const handled = handler(event);
      if (handled) {
        event.stopPropagation();
      }
    }
  });

  // Log available shortcuts for debugging
  console.log('[Shortcuts] Initialized:', Object.keys(shortcuts).join(', '));
})();
