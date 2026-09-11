(function () {
  'use strict';

  // ── State ─────────────────────────────────────────────────────────────
  var state = {
    accounts: [],
    categories: [],
    month: { year: null, month: null },
  };

  var today = new Date();
  state.month.year = today.getFullYear();
  state.month.month = today.getMonth() + 1;

  // ── API helper ────────────────────────────────────────────────────────
  function api(path, options) {
    options = options || {};
    var opts = {
      method: options.method || 'GET',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
    };
    if (options.body !== undefined) opts.body = JSON.stringify(options.body);

    return fetch(path, opts).then(function (res) {
      if (res.status === 204) return null;
      return res.json().then(function (data) {
        if (!res.ok) throw new Error(data && data.error ? data.error : 'Fehler');
        return data;
      });
    });
  }

  // ── Formatting ────────────────────────────────────────────────────────
  var euroFormatter = new Intl.NumberFormat('de-DE', { style: 'currency', currency: 'EUR' });

  function formatCents(cents) {
    return euroFormatter.format(cents / 100);
  }

  function pad(n) { return n < 10 ? '0' + n : '' + n; }

  function dateStr(d) {
    return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate());
  }

  var weekdayFormatter = new Intl.DateTimeFormat('de-DE', { weekday: 'long', day: 'numeric', month: 'long' });
  var monthFormatter = new Intl.DateTimeFormat('de-DE', { month: 'long', year: 'numeric' });

  function dayLabel(isoDate) {
    var parts = isoDate.split('-').map(Number);
    var d = new Date(parts[0], parts[1] - 1, parts[2]);
    var todayIso = dateStr(new Date());
    var yesterday = new Date();
    yesterday.setDate(yesterday.getDate() - 1);
    var yesterdayIso = dateStr(yesterday);

    if (isoDate === todayIso) return 'Heute';
    if (isoDate === yesterdayIso) return 'Gestern';
    return weekdayFormatter.format(d);
  }

  function esc(str) {
    var div = document.createElement('div');
    div.textContent = str == null ? '' : str;
    return div.innerHTML;
  }

  // ── Tabs ──────────────────────────────────────────────────────────────
  function initTabs() {
    var buttons = document.querySelectorAll('.tab-btn');
    buttons.forEach(function (btn) {
      btn.addEventListener('click', function () {
        var tab = btn.dataset.tab;
        buttons.forEach(function (b) {
          b.classList.toggle('active', b === btn);
          b.setAttribute('aria-selected', b === btn ? 'true' : 'false');
        });
        document.querySelectorAll('.tab-panel').forEach(function (panel) {
          panel.hidden = panel.id !== 'tab-' + tab;
        });
        if (tab === 'month') loadMonth();
        if (tab === 'settings') loadSettings();
      });
    });
  }

  // ── Overview ──────────────────────────────────────────────────────────
  function loadOverview() {
    return api('/api/overview').then(function (data) {
      renderBalance(data.totalBalanceCents);
      renderAccounts(data.accounts);
      renderTransactions(data.transactions);
    });
  }

  function renderBalance(totalCents) {
    var el = document.getElementById('total-balance');
    el.textContent = formatCents(totalCents);
    el.classList.toggle('amount-income', totalCents >= 0);
    el.classList.toggle('amount-expense', totalCents < 0);
  }

  function renderAccounts(accounts) {
    var grid = document.getElementById('accounts-grid');
    if (accounts.length === 0) {
      grid.innerHTML = '<p class="empty-hint">Noch keine Konten angelegt. Unter „Einstellungen" hinzufügen.</p>';
      return;
    }
    grid.innerHTML = accounts.map(function (a) {
      return '<div class="account-tile">' +
        '<span class="account-tile-name">' + (a.icon ? esc(a.icon) + ' ' : '') + esc(a.name) + '</span>' +
        '<span class="account-tile-balance ' + (a.balanceCents >= 0 ? 'amount-income' : 'amount-expense') + '">' +
        formatCents(a.balanceCents) + '</span>' +
        '</div>';
    }).join('');
  }

  function renderTransactions(transactions) {
    var container = document.getElementById('transactions-list');
    if (transactions.length === 0) {
      container.innerHTML = '<p class="empty-hint">Noch keine Buchungen vorhanden.</p>';
      return;
    }

    var groups = [];
    var byDate = {};
    transactions.forEach(function (t) {
      if (!byDate[t.date]) {
        byDate[t.date] = [];
        groups.push(t.date);
      }
      byDate[t.date].push(t);
    });

    container.innerHTML = groups.map(function (date) {
      var rows = byDate[date].map(renderTransactionRow).join('');
      return '<div class="transaction-day-group">' +
        '<div class="transaction-day-label">' + esc(dayLabel(date)) + '</div>' +
        rows +
        '</div>';
    }).join('');

    container.querySelectorAll('.transaction-row').forEach(function (row) {
      row.addEventListener('click', function () {
        var id = Number(row.dataset.id);
        var tx = transactions.find(function (t) { return t.id === id; });
        if (tx) openTransactionDialog(tx);
      });
    });
  }

  function renderTransactionRow(t) {
    var isIncome = t.categoryType === 'income';
    var sign = isIncome ? '+' : '−';
    return '<div class="transaction-row" data-id="' + t.id + '">' +
      '<div class="transaction-icon" style="background:' + esc(t.categoryColor) + '">' +
      (isIncome ? '↓' : '↑') +
      '</div>' +
      '<div class="transaction-details">' +
      '<div class="transaction-title">' +
      '<span>' + esc(t.description || t.categoryName) + '</span>' +
      (t.source === 'recurring' ? '<span class="transaction-recurring-icon" title="Wiederkehrende Buchung">↻</span>' : '') +
      '</div>' +
      '<div class="transaction-meta">' + esc(t.categoryName) + ' · ' + esc(t.accountName) + '</div>' +
      '</div>' +
      '<div class="transaction-amount ' + (isIncome ? 'amount-income' : 'amount-expense') + '">' +
      sign + ' ' + formatCents(t.amountCents) +
      '</div>' +
      '</div>';
  }

  // ── Month tab ─────────────────────────────────────────────────────────
  function loadMonth() {
    var label = document.getElementById('month-label');
    label.textContent = monthFormatter.format(new Date(state.month.year, state.month.month - 1, 1));

    return api('/api/month?year=' + state.month.year + '&month=' + state.month.month).then(function (data) {
      document.getElementById('month-income').textContent = formatCents(data.incomeCents);
      document.getElementById('month-expense').textContent = formatCents(data.expenseCents);

      var total = data.incomeCents + data.expenseCents;
      var incomePct = total > 0 ? (data.incomeCents / total) * 100 : 0;
      var expensePct = total > 0 ? (data.expenseCents / total) * 100 : 0;
      document.getElementById('month-bar-income').style.width = incomePct + '%';
      document.getElementById('month-bar-expense').style.width = expensePct + '%';

      var balanceEl = document.getElementById('month-balance');
      balanceEl.textContent = formatCents(data.balanceCents);
      balanceEl.classList.toggle('amount-income', data.balanceCents >= 0);
      balanceEl.classList.toggle('amount-expense', data.balanceCents < 0);
    });
  }

  function initMonthNav() {
    document.getElementById('month-prev').addEventListener('click', function () {
      shiftMonth(-1);
    });
    document.getElementById('month-next').addEventListener('click', function () {
      shiftMonth(1);
    });
  }

  function shiftMonth(delta) {
    var m = state.month.month + delta;
    var y = state.month.year;
    if (m < 1) { m = 12; y -= 1; }
    if (m > 12) { m = 1; y += 1; }
    state.month.year = y;
    state.month.month = m;
    loadMonth();
  }

  // ── Transaction dialog ────────────────────────────────────────────────
  function populateSelect(select, items, labelFn) {
    select.innerHTML = items.map(function (item) {
      return '<option value="' + item.id + '">' + esc(labelFn(item)) + '</option>';
    }).join('');
  }

  function openTransactionDialog(tx) {
    var dialog = document.getElementById('transaction-dialog');
    var form = document.getElementById('transaction-form');
    document.getElementById('transaction-dialog-title').textContent = tx ? 'Buchung bearbeiten' : 'Buchung hinzufügen';
    document.getElementById('transaction-error').hidden = true;

    populateSelect(document.getElementById('tx-category'), state.categories, function (c) { return c.name; });
    populateSelect(document.getElementById('tx-account'), state.accounts, function (a) { return a.name; });

    form.elements['id'].value = tx ? tx.id : '';
    form.elements['amount'].value = tx ? (tx.amountCents / 100).toFixed(2) : '';
    form.elements['date'].value = tx ? tx.date : dateStr(new Date());
    form.elements['description'].value = tx ? tx.description : '';
    if (tx) form.elements['categoryId'].value = tx.categoryId;
    if (tx) form.elements['accountId'].value = tx.accountId;

    document.getElementById('tx-delete').hidden = !tx;

    dialog.showModal();
  }

  function initTransactionDialog() {
    var dialog = document.getElementById('transaction-dialog');
    var form = document.getElementById('transaction-form');

    document.getElementById('add-transaction-btn').addEventListener('click', function () {
      if (state.accounts.length === 0) {
        alert('Bitte zuerst unter „Einstellungen" ein Konto anlegen.');
        return;
      }
      openTransactionDialog(null);
    });

    document.getElementById('tx-cancel').addEventListener('click', function () {
      dialog.close();
    });

    document.getElementById('tx-delete').addEventListener('click', function () {
      var id = form.elements['id'].value;
      if (!id) return;
      if (!confirm('Buchung wirklich löschen?')) return;
      api('/api/transactions/' + id, { method: 'DELETE' }).then(function () {
        dialog.close();
        loadOverview();
      }).catch(showFormError);
    });

    form.addEventListener('submit', function (e) {
      e.preventDefault();
      var id = form.elements['id'].value;
      var payload = {
        amount: form.elements['amount'].value,
        date: form.elements['date'].value,
        description: form.elements['description'].value,
        categoryId: Number(form.elements['categoryId'].value),
        accountId: Number(form.elements['accountId'].value),
      };
      var req = id
        ? api('/api/transactions/' + id, { method: 'PUT', body: payload })
        : api('/api/transactions', { method: 'POST', body: payload });

      req.then(function () {
        dialog.close();
        loadOverview();
      }).catch(showFormError);
    });
  }

  function showFormError(err) {
    var el = document.getElementById('transaction-error');
    el.textContent = err.message;
    el.hidden = false;
  }

  // ── Settings: accounts / categories / recurring ──────────────────────
  function loadSettings() {
    return Promise.all([loadAccountsSettings(), loadCategoriesSettings(), loadRecurringSettings()]);
  }

  function refreshReferenceData() {
    return Promise.all([
      api('/api/accounts').then(function (a) { state.accounts = a; }),
      api('/api/categories').then(function (c) { state.categories = c; }),
    ]);
  }

  function loadAccountsSettings() {
    return api('/api/accounts').then(function (accounts) {
      state.accounts = accounts;
      var list = document.getElementById('settings-accounts');
      list.innerHTML = accounts.map(function (a) {
        return '<li data-id="' + a.id + '">' +
          '<span class="settings-item-name">' + (a.icon ? esc(a.icon) + ' ' : '') + esc(a.name) + '</span>' +
          '<button type="button" class="btn-icon settings-delete" aria-label="Konto löschen">🗑</button>' +
          '</li>';
      }).join('');
      list.querySelectorAll('.settings-delete').forEach(function (btn) {
        btn.addEventListener('click', function () {
          var id = btn.closest('li').dataset.id;
          if (!confirm('Konto wirklich löschen?')) return;
          api('/api/accounts/' + id, { method: 'DELETE' })
            .then(loadAccountsSettings)
            .catch(function (err) { alert(err.message); });
        });
      });
    });
  }

  function loadCategoriesSettings() {
    return api('/api/categories').then(function (categories) {
      state.categories = categories;
      var list = document.getElementById('settings-categories');
      list.innerHTML = categories.map(function (c) {
        return '<li data-id="' + c.id + '">' +
          '<span class="color-dot" style="background:' + esc(c.color) + '"></span>' +
          '<span class="settings-item-name">' + esc(c.name) + '</span>' +
          '<span class="transaction-meta">' + (c.type === 'income' ? 'Einnahme' : 'Ausgabe') + '</span>' +
          '<button type="button" class="btn-icon settings-delete" aria-label="Kategorie löschen">🗑</button>' +
          '</li>';
      }).join('');
      list.querySelectorAll('.settings-delete').forEach(function (btn) {
        btn.addEventListener('click', function () {
          var id = btn.closest('li').dataset.id;
          if (!confirm('Kategorie wirklich löschen?')) return;
          api('/api/categories/' + id, { method: 'DELETE' })
            .then(loadCategoriesSettings)
            .catch(function (err) { alert(err.message); });
        });
      });
    });
  }

  function loadRecurringSettings() {
    return Promise.all([api('/api/recurring'), refreshReferenceData()]).then(function (results) {
      var templates = results[0];
      populateRecurringFormSelects();
      var list = document.getElementById('settings-recurring');
      if (templates.length === 0) {
        list.innerHTML = '<li>Keine wiederkehrenden Buchungen angelegt.</li>';
        return;
      }
      list.innerHTML = templates.map(function (rt) {
        var category = state.categories.find(function (c) { return c.id === rt.categoryId; });
        var account = state.accounts.find(function (a) { return a.id === rt.accountId; });
        return '<li data-id="' + rt.id + '">' +
          '<span class="settings-item-name">' + esc(rt.name) + ' (' + formatCents(rt.amountCents) + ')</span>' +
          '<span class="transaction-meta">' + esc(category ? category.name : '') + ' · ' + esc(account ? account.name : '') +
          ' · ' + esc(rt.interval) + ' · ab ' + esc(rt.nextDueDate) + '</span>' +
          '<label><input type="checkbox" class="recurring-active" ' + (rt.active ? 'checked' : '') + '> aktiv</label>' +
          '<button type="button" class="btn-icon settings-delete" aria-label="Vorlage löschen">🗑</button>' +
          '</li>';
      }).join('');

      list.querySelectorAll('.recurring-active').forEach(function (checkbox) {
        checkbox.addEventListener('change', function () {
          var li = checkbox.closest('li');
          var id = li.dataset.id;
          var rt = templates.find(function (t) { return String(t.id) === id; });
          api('/api/recurring/' + id, {
            method: 'PUT',
            body: {
              name: rt.name,
              amount: (rt.amountCents / 100).toFixed(2),
              categoryId: rt.categoryId,
              accountId: rt.accountId,
              interval: rt.interval,
              nextDueDate: rt.nextDueDate,
              active: checkbox.checked,
            },
          }).catch(function (err) { alert(err.message); });
        });
      });

      list.querySelectorAll('.settings-delete').forEach(function (btn) {
        btn.addEventListener('click', function () {
          var id = btn.closest('li').dataset.id;
          if (!confirm('Vorlage wirklich löschen?')) return;
          api('/api/recurring/' + id, { method: 'DELETE' })
            .then(loadRecurringSettings)
            .catch(function (err) { alert(err.message); });
        });
      });
    });
  }

  function initSettingsForms() {
    document.getElementById('account-form').addEventListener('submit', function (e) {
      e.preventDefault();
      var form = e.target;
      api('/api/accounts', {
        method: 'POST',
        body: { name: form.elements['name'].value, icon: form.elements['icon'].value },
      }).then(function () {
        form.reset();
        return loadAccountsSettings();
      }).catch(function (err) { alert(err.message); });
    });

    document.getElementById('category-form').addEventListener('submit', function (e) {
      e.preventDefault();
      var form = e.target;
      api('/api/categories', {
        method: 'POST',
        body: {
          name: form.elements['name'].value,
          color: form.elements['color'].value,
          type: form.elements['type'].value,
        },
      }).then(function () {
        form.reset();
        return loadCategoriesSettings();
      }).catch(function (err) { alert(err.message); });
    });

    document.getElementById('recurring-form').addEventListener('submit', function (e) {
      e.preventDefault();
      var form = e.target;
      api('/api/recurring', {
        method: 'POST',
        body: {
          name: form.elements['name'].value,
          amount: form.elements['amount'].value,
          categoryId: Number(form.elements['categoryId'].value),
          accountId: Number(form.elements['accountId'].value),
          interval: form.elements['interval'].value,
          nextDueDate: form.elements['nextDueDate'].value,
          active: true,
        },
      }).then(function () {
        form.reset();
        return loadRecurringSettings();
      }).catch(function (err) { alert(err.message); });
    });
  }

  function populateRecurringFormSelects() {
    var categorySelect = document.querySelector('#recurring-form select[name="categoryId"]');
    var accountSelect = document.querySelector('#recurring-form select[name="accountId"]');
    populateSelect(categorySelect, state.categories, function (c) { return c.name; });
    populateSelect(accountSelect, state.accounts, function (a) { return a.name; });
  }

  // ── Init ──────────────────────────────────────────────────────────────
  document.addEventListener('DOMContentLoaded', function () {
    initTabs();
    initMonthNav();
    initTransactionDialog();
    initSettingsForms();
    refreshReferenceData().then(loadOverview);
  });
})();
