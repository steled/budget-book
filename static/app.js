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
      renderUpcoming(data.upcoming || []);
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
      var id = Number(row.dataset.id);
      var tx = transactions.find(function (t) { return t.id === id; });
      if (tx) row.querySelector('.transaction-icon').style.background = tx.categoryColor;
      row.addEventListener('click', function () {
        if (tx) openTransactionDialog(tx);
      });
    });
  }

  function renderUpcoming(items) {
    var container = document.getElementById('upcoming-list');
    container.hidden = items.length === 0;
    if (items.length === 0) {
      container.innerHTML = '';
      return;
    }
    var total = items.reduce(function (sum, t) {
      return sum + (t.categoryType === 'income' ? t.amountCents : -t.amountCents);
    }, 0);
    container.innerHTML = '<div class="transaction-day-label">Ausstehend im ' +
      esc(new Intl.DateTimeFormat('de-DE', { month: 'long' }).format(new Date())) +
      ' (' + esc(formatCents(total)) + ')</div>' +
      items.map(function (t) {
        var isIncome = t.categoryType === 'income';
        var parts = t.date.split('-');
        return '<div class="transaction-row transaction-pending">' +
          '<div class="transaction-icon">' + (isIncome ? '↓' : '↑') + '</div>' +
          '<div class="transaction-details">' +
          '<div class="transaction-title"><span>' + esc(t.description) + '</span>' +
          '<span class="transaction-recurring-icon" title="Wiederkehrende Buchung">↻</span></div>' +
          '<div class="transaction-meta">fällig am ' + esc(parts[2] + '.' + parts[1] + '.') + ' · ' +
          esc(t.categoryName) + ' · ' + esc(t.accountName) + '</div>' +
          '</div>' +
          '<div class="transaction-amount ' + (isIncome ? 'amount-income' : 'amount-expense') + '">' +
          (isIncome ? '+' : '−') + ' ' + formatCents(t.amountCents) + '</div>' +
          '</div>';
      }).join('');
    container.querySelectorAll('.transaction-pending').forEach(function (row, i) {
      row.querySelector('.transaction-icon').style.background = items[i].categoryColor;
    });
  }

  function renderTransactionRow(t) {
    var isIncome = t.categoryType === 'income';
    var sign = isIncome ? '+' : '−';
    return '<div class="transaction-row" data-id="' + t.id + '">' +
      // Colour is set via CSSOM after rendering: a style="" attribute is
      // blocked by the CSP (style-src 'self').
      '<div class="transaction-icon">' +
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
      populateRecurringFormSelects();
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
      populateRecurringFormSelects();
      var list = document.getElementById('settings-categories');
      list.innerHTML = categories.map(function (c) {
        return '<li data-id="' + c.id + '">' +
          '<span class="color-dot"></span>' +
          '<span class="settings-item-name">' + esc(c.name) + '</span>' +
          '<span class="transaction-meta">' + (c.type === 'income' ? 'Einnahme' : 'Ausgabe') + '</span>' +
          '<button type="button" class="btn-icon settings-delete" aria-label="Kategorie löschen">🗑</button>' +
          '</li>';
      }).join('');
      list.querySelectorAll('.color-dot').forEach(function (dot, i) {
        dot.style.background = categories[i].color;
      });
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

  var INTERVAL_LABELS = { weekly: 'wöchentlich', monthly: 'monatlich', yearly: 'jährlich' };

  function recurringScheduleLabel(rt) {
    var label = INTERVAL_LABELS[rt.interval] || rt.interval;
    if (rt.remainingCount === null || rt.remainingCount === undefined) {
      return label + ' · ab ' + rt.nextDueDate;
    }
    if (rt.remainingCount === 0) return 'abgeschlossen';
    if (rt.remainingCount === 1) return 'einmalig am ' + rt.nextDueDate;
    return label + ' · noch ' + rt.remainingCount + ' Raten (' +
      formatCents(rt.outstandingCents) + ' offen) · nächste ' + rt.nextDueDate;
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
          '<span class="transaction-meta recurring-meta">' + esc(category ? category.name : '') + ' · ' + esc(account ? account.name : '') +
          ' · ' + esc(recurringScheduleLabel(rt)) + '</span>' +
          '<label><input type="checkbox" class="recurring-active" ' + (rt.active ? 'checked' : '') + '> aktiv</label>' +
          (rt.outstandingCents > 0 ? '<button type="button" class="btn btn-secondary recurring-extra">Extrazahlung</button>' : '') +
          '<button type="button" class="btn-icon recurring-edit" aria-label="Vorlage bearbeiten">✏️</button>' +
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
              remainingCount: rt.remainingCount,
              active: checkbox.checked,
            },
          }).catch(function (err) { alert(err.message); });
        });
      });

      list.querySelectorAll('.recurring-extra').forEach(function (btn) {
        btn.addEventListener('click', function () {
          var id = btn.closest('li').dataset.id;
          openExtraDialog(templates.find(function (t) { return String(t.id) === id; }));
        });
      });

      list.querySelectorAll('.recurring-edit').forEach(function (btn) {
        btn.addEventListener('click', function () {
          var id = btn.closest('li').dataset.id;
          startRecurringEdit(templates.find(function (t) { return String(t.id) === id; }));
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

  var editingRecurring = null;

  function setRecurringEditMode(rt) {
    var form = document.getElementById('recurring-form');
    editingRecurring = rt;
    document.getElementById('recurring-submit').textContent = rt ? 'Speichern' : 'Vorlage hinzufügen';
    document.getElementById('recurring-cancel').hidden = !rt;
    if (!rt) {
      form.reset();
      form.elements['remainingCount'].disabled = false;
    }
  }

  function startRecurringEdit(rt) {
    var form = document.getElementById('recurring-form');
    var once = rt.remainingCount === 1;
    form.elements['name'].value = rt.name;
    form.elements['amount'].value = (rt.amountCents / 100).toFixed(2);
    form.elements['categoryId'].value = rt.categoryId;
    form.elements['accountId'].value = rt.accountId;
    form.elements['interval'].value = once ? 'once' : rt.interval;
    form.elements['remainingCount'].disabled = once;
    form.elements['remainingCount'].value = once || rt.remainingCount === null ? '' : rt.remainingCount;
    form.elements['nextDueDate'].value = rt.nextDueDate;
    setRecurringEditMode(rt);
    form.elements['name'].focus();
  }

  function openExtraDialog(rt) {
    var form = document.getElementById('extra-form');
    form.reset();
    form.elements['id'].value = rt.id;
    form.elements['amount'].max = (rt.outstandingCents / 100).toFixed(2);
    form.elements['date'].value = dateStr(new Date());
    document.getElementById('extra-info').textContent = rt.name + ': offen ' + formatCents(rt.outstandingCents);
    document.getElementById('extra-error').hidden = true;
    document.getElementById('extra-dialog').showModal();
  }

  function initExtraDialog() {
    var dialog = document.getElementById('extra-dialog');
    var form = document.getElementById('extra-form');
    document.getElementById('extra-cancel').addEventListener('click', function () { dialog.close(); });
    form.addEventListener('submit', function (e) {
      e.preventDefault();
      api('/api/recurring/' + form.elements['id'].value + '/extra', {
        method: 'POST',
        body: { amount: form.elements['amount'].value, date: form.elements['date'].value },
      }).then(function () {
        dialog.close();
        loadRecurringSettings();
        loadOverview();
      }).catch(function (err) {
        var el = document.getElementById('extra-error');
        el.textContent = err.message;
        el.hidden = false;
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

    var recurringForm = document.getElementById('recurring-form');
    recurringForm.elements['interval'].addEventListener('change', function (e) {
      var countInput = recurringForm.elements['remainingCount'];
      var once = e.target.value === 'once';
      countInput.disabled = once;
      if (once) countInput.value = '';
    });

    document.getElementById('recurring-cancel').addEventListener('click', function () {
      setRecurringEditMode(null);
    });

    recurringForm.addEventListener('submit', function (e) {
      e.preventDefault();
      var form = e.target;
      var once = form.elements['interval'].value === 'once';
      var countRaw = form.elements['remainingCount'].value;
      var editing = editingRecurring;
      api(editing ? '/api/recurring/' + editing.id : '/api/recurring', {
        method: editing ? 'PUT' : 'POST',
        body: {
          name: form.elements['name'].value,
          amount: form.elements['amount'].value,
          categoryId: Number(form.elements['categoryId'].value),
          accountId: Number(form.elements['accountId'].value),
          interval: once ? 'monthly' : form.elements['interval'].value,
          remainingCount: once ? 1 : (countRaw === '' ? null : Number(countRaw)),
          nextDueDate: form.elements['nextDueDate'].value,
          active: editing ? editing.active : true,
        },
      }).then(function () {
        setRecurringEditMode(null);
        return loadRecurringSettings();
      }).catch(function (err) { alert(err.message); });
    });
  }

  function populateRecurringFormSelects() {
    var categorySelect = document.querySelector('#recurring-form select[name="categoryId"]');
    var accountSelect = document.querySelector('#recurring-form select[name="accountId"]');
    var categoryValue = categorySelect.value;
    var accountValue = accountSelect.value;
    populateSelect(categorySelect, state.categories, function (c) { return c.name; });
    populateSelect(accountSelect, state.accounts, function (a) { return a.name; });
    if (categoryValue) categorySelect.value = categoryValue;
    if (accountValue) accountSelect.value = accountValue;
  }

  // ── Init ──────────────────────────────────────────────────────────────
  document.addEventListener('DOMContentLoaded', function () {
    initTabs();
    initMonthNav();
    initTransactionDialog();
    initSettingsForms();
    initExtraDialog();
    refreshReferenceData().then(loadOverview);
  });
})();
