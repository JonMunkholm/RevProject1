(function () {
  const statusEl = document.querySelector('[data-dashboard-status]');
  const refreshButton = document.querySelector('#refresh-dashboard');
  const tableBody = document.querySelector('#recent-contracts tbody');
  const nav = document.querySelector('[data-dashboard-nav]');
  const navLinks = nav ? Array.from(nav.querySelectorAll('.side-nav__link')) : [];
  const anchorLinks = navLinks.filter((link) => link.tagName === 'A');
  const logoutForm = document.querySelector('#logout-form');
  const logoutButton = document.querySelector('#logout-button');
  const frame = document.querySelector('[data-dashboard-endpoint]');
  const dataEndpoint = frame && frame.dataset && frame.dataset.dashboardEndpoint
    ? frame.dataset.dashboardEndpoint
    : '/api/dashboard/summary';

  const metricMap = {
    users: bindMetric('users'),
    customers: bindMetric('customers'),
    contracts: bindMetric('contracts'),
    products: bindMetric('products'),
    bundles: bindMetric('bundles'),
    performanceObligations: bindMetric('performance-obligations'),
  };

  function bindMetric(id) {
    const container = document.querySelector(`[data-metric="${id}"]`);
    return {
      container,
      value: container ? container.querySelector('.metric-value') : null,
      detail: container ? container.querySelector('.metric-detail-value') : null,
    };
  }

  function highlightActiveNav() {
    if (!anchorLinks.length) return;
    const pathname = window.location.pathname.replace(/^\//, '');
    let route = pathname.startsWith('app/') ? pathname.slice(4) : pathname;
    if (route === '') route = 'dashboard';

    anchorLinks.forEach((link) => {
      const linkRoute = link.dataset.route || '';
      const isMatch = route === linkRoute || route.startsWith(`${linkRoute}/`);
      if (isMatch) {
        link.setAttribute('aria-current', 'page');
      } else {
        link.removeAttribute('aria-current');
      }
    });
  }

  highlightActiveNav();

  anchorLinks.forEach((link) => {
    link.addEventListener('click', () => {
      anchorLinks.forEach((item) => item.removeAttribute('aria-current'));
      link.setAttribute('aria-current', 'page');
    });
  });

  if (logoutForm && logoutButton) {
    logoutForm.addEventListener('submit', handleLogout, { once: true });
  }

  async function handleLogout(event) {
    event.preventDefault();

    logoutButton.disabled = true;

    try {
      const response = await fetch('/auth/logout', {
        method: 'POST',
        credentials: 'include',
        headers: {
          Accept: 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error(`Logout failed with status ${response.status}`);
      }
    } catch (err) {
      console.error('Failed to logout:', err);
    } finally {
      window.location.href = '/login';
    }
  }

  window.addEventListener('pageshow', (event) => {
    if (event.persisted) {
      window.location.replace('/login');
    }
  });

  function setStatus(message, isError) {
    if (!statusEl) return;
    statusEl.textContent = message;
    if (isError) {
      statusEl.dataset.error = 'true';
    } else {
      delete statusEl.dataset.error;
    }
  }

  function formatNumber(value) {
    return new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(value ?? 0);
  }

  function formatDate(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return '—';
    }
    return date.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    });
  }

  function formatRelative(value) {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return '—';
    }

    const diff = Date.now() - date.getTime();
    const minutes = Math.round(diff / 60000);
    if (minutes < 1) return 'just now';
    if (minutes < 60) return `${minutes} min ago`;
    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours} hr${hours > 1 ? 's' : ''} ago`;
    const days = Math.round(hours / 24);
    if (days < 30) return `${days} day${days > 1 ? 's' : ''} ago`;
    return date.toLocaleDateString();
  }

  function updateMetrics(metrics) {
    if (metricMap.users.value) {
      metricMap.users.value.textContent = formatNumber(metrics.totalUsers);
    }
    if (metricMap.users.detail) {
      metricMap.users.detail.textContent = formatNumber(metrics.activeUsers);
    }

    if (metricMap.customers.value) {
      metricMap.customers.value.textContent = formatNumber(metrics.totalCustomers);
    }
    if (metricMap.customers.detail) {
      metricMap.customers.detail.textContent = formatNumber(metrics.activeCustomers);
    }

    if (metricMap.contracts.value) {
      metricMap.contracts.value.textContent = formatNumber(metrics.totalContracts);
    }
    if (metricMap.contracts.detail) {
      metricMap.contracts.detail.textContent = formatNumber(metrics.finalizedContracts);
    }

    if (metricMap.products.value) {
      metricMap.products.value.textContent = formatNumber(metrics.totalProducts);
    }
    if (metricMap.products.detail) {
      metricMap.products.detail.textContent = formatNumber(metrics.activeProducts);
    }

    if (metricMap.bundles.value) {
      metricMap.bundles.value.textContent = formatNumber(metrics.totalBundles);
    }
    if (metricMap.bundles.detail) {
      metricMap.bundles.detail.textContent = formatNumber(metrics.activeBundles);
    }

    if (metricMap.performanceObligations.value) {
      metricMap.performanceObligations.value.textContent = formatNumber(metrics.performanceObligations);
    }
  }

  function renderContracts(contracts) {
    if (!tableBody) return;
    tableBody.innerHTML = '';

    if (!contracts || contracts.length === 0) {
      const row = document.createElement('tr');
      row.className = 'placeholder-row';
      const cell = document.createElement('td');
      cell.colSpan = 5;
      cell.textContent = 'No recent contract updates yet.';
      row.appendChild(cell);
      tableBody.appendChild(row);
      return;
    }

    for (const contract of contracts) {
      const row = document.createElement('tr');

      row.appendChild(createCell(contract.customerName, 'Customer'));
      row.appendChild(createCell(formatDate(contract.startDate), 'Start'));
      row.appendChild(createCell(formatDate(contract.endDate), 'End'));

      const statusCell = createCell('', 'Status');
      const chip = document.createElement('span');
      chip.className = 'status-chip' + (contract.isFinal ? ' is-final' : '');
      chip.textContent = contract.isFinal ? 'Finalized' : 'Draft';
      statusCell.appendChild(chip);
      row.appendChild(statusCell);

      row.appendChild(createCell(formatRelative(contract.updatedAt), 'Updated'));

      tableBody.appendChild(row);
    }
  }

  function createCell(value, label) {
    const cell = document.createElement('td');
    if (label) {
      cell.setAttribute('data-label', label);
    }
    cell.textContent = value;
    return cell;
  }

  async function loadDashboard() {
    setStatus('Refreshing…');
    if (refreshButton) {
      refreshButton.disabled = true;
    }

    try {
      const response = await fetch(dataEndpoint || '/api/dashboard/summary', {
        credentials: 'include',
        headers: {
          Accept: 'application/json',
        },
      });

      if (response.status === 401) {
        window.location.href = '/login';
        return;
      }

      if (!response.ok) {
        throw new Error(`Request failed with status ${response.status}`);
      }

      const payload = await response.json();
      updateMetrics(payload.metrics ?? {});
      renderContracts(payload.recentContracts ?? []);
      setStatus(`Updated ${new Date().toLocaleTimeString()}`);

      document.body.classList.remove('is-loading');
    } catch (err) {
      console.error('Dashboard fetch failed:', err);
      setStatus('Unable to load dashboard data. Retry shortly.', true);
    } finally {
      if (refreshButton) {
        refreshButton.disabled = false;
      }
    }
  }

  if (refreshButton) {
    refreshButton.addEventListener('click', loadDashboard);
  }

  document.addEventListener('visibilitychange', () => {
    if (!document.hidden && !document.body.classList.contains('is-loading')) {
      loadDashboard();
    }
  });

  loadDashboard();
})();
