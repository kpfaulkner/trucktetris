// App orchestration: tab switching, CRUD forms, selection, and solving.
import { createViewer, colourFor } from '/viewer.js';

// --- tiny helpers ------------------------------------------------------------

const $ = (sel) => document.querySelector(sel);
const el = (tag, props = {}, kids = []) => {
  const n = Object.assign(document.createElement(tag), props);
  for (const k of kids) n.append(k);
  return n;
};

async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(path, opts);
  if (!res.ok) {
    const msg = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(msg.error || `HTTP ${res.status}`);
  }
  return res.status === 204 ? null : res.json();
}

function setError(msg) {
  $('#error').textContent = msg || '';
}

// --- state -------------------------------------------------------------------

let cases = [];
let trucks = [];
let viewer = null;

async function refreshData() {
  [cases, trucks] = await Promise.all([api('GET', '/api/cases'), api('GET', '/api/trucks')]);
  renderCaseTable();
  renderTruckTable();
  renderSelection();
}

// --- tabs --------------------------------------------------------------------

function initTabs() {
  for (const btn of document.querySelectorAll('.tab')) {
    btn.addEventListener('click', () => {
      for (const b of document.querySelectorAll('.tab')) b.classList.remove('active');
      for (const p of document.querySelectorAll('.page')) p.classList.remove('active');
      btn.classList.add('active');
      $(`#page-${btn.dataset.page}`).classList.add('active');
      if (btn.dataset.page === 'plan' && viewer) viewer.resize();
    });
  }
}

// --- case management ---------------------------------------------------------

function renderCaseTable() {
  const tb = $('#case-rows');
  tb.replaceChildren();
  for (const c of cases) {
    const row = el('tr', {}, [
      el('td', { textContent: c.name }),
      el('td', { textContent: `${c.dim.l}×${c.dim.w}×${c.dim.h}` }),
      el('td', { textContent: c.weight }),
      el('td', { textContent: c.type }),
      el('td', { textContent: c.stackable ? 'yes' : 'no' }),
      el('td', { textContent: c.stackable ? c.maxStackWeight : '—' }),
      el('td', { textContent: c.canLieOnSide ? 'yes' : 'no' }),
    ]);
    const del = el('button', { textContent: 'Delete', className: 'small' });
    del.addEventListener('click', async () => {
      try { await api('DELETE', `/api/cases/${c.id}`); await refreshData(); }
      catch (e) { setError(e.message); }
    });
    row.append(el('td', {}, [del]));
    tb.append(row);
  }
}

function readCaseForm() {
  return {
    name: $('#c-name').value.trim(),
    type: $('#c-type').value.trim() || 'default',
    dim: { l: +$('#c-l').value, w: +$('#c-w').value, h: +$('#c-h').value },
    weight: +$('#c-weight').value,
    stackable: $('#c-stackable').checked,
    maxStackWeight: +$('#c-bears').value,
    stackableOn: $('#c-stack').value.split(',').map((s) => s.trim()).filter(Boolean),
    canLieOnSide: $('#c-side').checked,
  };
}

async function submitCase(e) {
  e.preventDefault();
  setError('');
  try {
    await api('POST', '/api/cases', readCaseForm());
    $('#case-form').reset();
    await refreshData();
  } catch (err) { setError(err.message); }
}

// --- truck management --------------------------------------------------------

function renderTruckTable() {
  const tb = $('#truck-rows');
  tb.replaceChildren();
  for (const t of trucks) {
    const axles = (t.axles || []).map((a) => `${a.position}mm/${a.maxLoad}kg`).join('; ');
    const row = el('tr', {}, [
      el('td', { textContent: t.name }),
      el('td', { textContent: `${t.dim.l}×${t.dim.w}×${t.dim.h}` }),
      el('td', { textContent: t.grossMax }),
      el('td', { textContent: axles }),
    ]);
    const del = el('button', { textContent: 'Delete', className: 'small' });
    del.addEventListener('click', async () => {
      try { await api('DELETE', `/api/trucks/${t.id}`); await refreshData(); }
      catch (e) { setError(e.message); }
    });
    row.append(el('td', {}, [del]));
    tb.append(row);
  }
}

// Parse "pos:load, pos:load" into an axle array.
function parseAxles(text) {
  return text.split(',').map((s) => s.trim()).filter(Boolean).map((pair) => {
    const [pos, load] = pair.split(':').map((n) => +n.trim());
    return { position: pos, maxLoad: load };
  });
}

async function submitTruck(e) {
  e.preventDefault();
  setError('');
  try {
    await api('POST', '/api/trucks', {
      name: $('#t-name').value.trim(),
      dim: { l: +$('#t-l').value, w: +$('#t-w').value, h: +$('#t-h').value },
      grossMax: +$('#t-gross').value,
      axles: parseAxles($('#t-axles').value),
    });
    $('#truck-form').reset();
    await refreshData();
  } catch (err) { setError(err.message); }
}

// --- plan / selection --------------------------------------------------------

function renderSelection() {
  const sel = $('#sel-truck');
  sel.replaceChildren(...trucks.map((t) => el('option', { value: t.id, textContent: t.name })));

  const list = $('#sel-cases');
  list.replaceChildren();
  for (const c of cases) {
    const cb = el('input', { type: 'checkbox', value: c.id, checked: true });
    cb.className = 'sel-case';
    const hex = colourFor(c.type).toString(16).padStart(6, '0');
    list.append(el('label', { className: 'sel-row' }, [
      cb,
      el('span', { className: 'swatch', style: `background:#${hex}` }),
      el('span', { textContent: `${c.name} (${c.weight}kg)` }),
    ]));
  }
}

async function solve() {
  setError('');
  const truckId = $('#sel-truck').value;
  const caseIds = [...document.querySelectorAll('.sel-case:checked')].map((c) => c.value);
  if (!truckId) { setError('Select a truck'); return; }
  try {
    const plan = await api('POST', '/api/solve', { truckId, caseIds });
    const caseById = new Map(cases.map((c) => [c.id, c]));
    viewer.render(plan, caseById);
    renderPlanStats(plan, caseById);
  } catch (err) { setError(err.message); }
}

function renderPlanStats(plan, caseById) {
  $('#stat-truck').textContent = plan.truck.name;
  $('#stat-placed').textContent = plan.summary.placedCount;
  $('#stat-unplaced').textContent = plan.summary.unplacedCount;
  $('#stat-weight').textContent = `${plan.summary.totalWeight} kg`;

  const axles = $('#stat-axles');
  axles.replaceChildren();
  if ((plan.axleLoads || []).length) {
    axles.append(el('b', { textContent: 'Axle loads' }));
    axles.append(el('div', {
      textContent: 'Orange hoops in the view mark axle positions.',
      style: 'font-size:0.75rem;color:#c60;padding:0.1rem 0 0.2rem;',
    }));
    plan.axleLoads.forEach((a, i) => {
      const row = el('div', { className: `axle${a.over ? ' over' : ''}` }, [
        el('span', { textContent: `Axle ${i + 1} @ ${a.position}mm` }),
        el('b', { textContent: `${a.load} / ${a.maxLoad} kg${a.over ? ' ⚠' : ''}` }),
      ]);
      axles.append(row);
    });
  }

  $('#stat-unfit').textContent = plan.unplaced.length
    ? `Did not fit: ${plan.unplaced.map((id) => caseById.get(id)?.name || id).join(', ')}`
    : '';
}

// --- boot --------------------------------------------------------------------

async function boot() {
  initTabs();
  viewer = createViewer($('#view'));
  $('#case-form').addEventListener('submit', submitCase);
  $('#truck-form').addEventListener('submit', submitTruck);
  $('#solve').addEventListener('click', solve);
  try {
    await refreshData();
    await solve(); // show an initial plan
  } catch (err) { setError(err.message); }
}

boot();
