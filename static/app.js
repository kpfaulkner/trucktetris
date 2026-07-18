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

// Current plan context, so manual edits can be re-evaluated and survive tab
// switches within the session.
let planTruckId = null;
let planCaseById = new Map();

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
    const edit = el('button', { textContent: 'Edit', className: 'small edit' });
    edit.addEventListener('click', () => startEditCase(c));
    const del = el('button', { textContent: 'Delete', className: 'small' });
    del.addEventListener('click', async () => {
      try {
        await api('DELETE', `/api/cases/${c.id}`);
        if (editingCaseId === c.id) resetCaseForm();
        await refreshData();
      } catch (e) { setError(e.message); }
    });
    row.append(el('td', {}, [edit, del]));
    tb.append(row);
  }
}

// --- case edit state ---------------------------------------------------------

let editingCaseId = null;

function startEditCase(c) {
  editingCaseId = c.id;
  $('#c-name').value = c.name;
  $('#c-type').value = c.type;
  $('#c-l').value = c.dim.l;
  $('#c-w').value = c.dim.w;
  $('#c-h').value = c.dim.h;
  $('#c-weight').value = c.weight;
  $('#c-bears').value = c.maxStackWeight;
  $('#c-stack').value = (c.stackableOn || []).join(', ');
  $('#c-stackable').checked = !!c.stackable;
  $('#c-side').checked = !!c.canLieOnSide;
  $('#case-submit').textContent = 'Save changes';
  $('#case-cancel').hidden = false;
  $('#c-name').focus();
}

function resetCaseForm() {
  editingCaseId = null;
  $('#case-form').reset();
  $('#case-submit').textContent = 'Add case';
  $('#case-cancel').hidden = true;
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
    if (editingCaseId) {
      await api('PUT', `/api/cases/${editingCaseId}`, readCaseForm());
    } else {
      await api('POST', '/api/cases', readCaseForm());
    }
    resetCaseForm();
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
      el('td', { textContent: t.heavyThreshold || '—' }),
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
      heavyThreshold: +$('#t-heavy').value,
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
    const bears = c.stackable ? `bears ${c.maxStackWeight}kg` : 'no stacking';
    list.append(el('label', { className: 'sel-row' }, [
      cb,
      el('span', { className: 'swatch', style: `background:#${hex}` }),
      el('span', {}, [
        el('span', { textContent: c.name }),
        el('span', {
          className: 'sel-meta',
          textContent: ` — ${c.weight}kg, ${bears}`,
        }),
      ]),
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
    planTruckId = truckId;
    planCaseById = new Map(cases.map((c) => [c.id, c]));
    // Re-solving discards any manual edits and overrides with the solver result.
    viewer.render(plan, planCaseById, { onChange: onPlacementsChanged });
    renderPlanStats(plan, planCaseById);
  } catch (err) { setError(err.message); }
}

// Live evaluation while dragging, throttled to one in-flight request. A drag
// that lands mid-request re-runs once the request returns.
let evalPending = false;
let evalQueued = null;

function onPlacementsChanged(placements) {
  if (evalPending) { evalQueued = placements; return; }
  evalPending = true;
  api('POST', '/api/evaluate', { truckId: planTruckId, placements })
    .then((ev) => {
      viewer.applyEvaluation(ev);
      updateLiveStats(ev);
    })
    .catch((err) => setError(err.message))
    .finally(() => {
      evalPending = false;
      if (evalQueued) { const q = evalQueued; evalQueued = null; onPlacementsChanged(q); }
    });
}

function axleRows(axleLoads) {
  const axles = $('#stat-axles');
  axles.replaceChildren();
  if (!(axleLoads || []).length) return;
  axles.append(el('b', { textContent: 'Axle loads' }));
  axles.append(el('div', {
    textContent: 'Orange hoops mark axles. Drag a box to reposition; drag it over another box to stack on top.',
    style: 'font-size:0.75rem;color:#c60;padding:0.1rem 0 0.2rem;',
  }));
  axleLoads.forEach((a, i) => {
    axles.append(el('div', { className: `axle${a.over ? ' over' : ''}` }, [
      el('span', { textContent: `Axle ${i + 1} @ ${a.position}mm` }),
      el('b', { textContent: `${a.load} / ${a.maxLoad} kg${a.over ? ' ⚠' : ''}` }),
    ]));
  });
}

function renderPlanStats(plan, caseById) {
  $('#stat-truck').textContent = plan.truck.name;
  $('#stat-placed').textContent = plan.summary.placedCount;
  $('#stat-unplaced').textContent = plan.summary.unplacedCount;
  $('#stat-weight').textContent = `${plan.summary.totalWeight} kg`;
  axleRows(plan.axleLoads);
  $('#stat-violations').textContent = '';
  $('#stat-unfit').textContent = plan.unplaced.length
    ? `Did not fit: ${plan.unplaced.map((id) => caseById.get(id)?.name || id).join(', ')}`
    : '';
}

// updateLiveStats refreshes the panel from a manual-edit evaluation.
function updateLiveStats(ev) {
  const gross = ev.overGross ? ' ⚠ over gross' : '';
  $('#stat-weight').textContent = `${ev.totalWeight} kg${gross}`;
  axleRows(ev.axleLoads);

  const problems = [];
  if (ev.overGross) problems.push('over gross weight');
  if ((ev.axleLoads || []).some((a) => a.over)) problems.push('axle overloaded');
  if ((ev.collisions || []).length) problems.push(`${ev.collisions.length} overlapping`);
  if ((ev.outOfBounds || []).length) problems.push(`${ev.outOfBounds.length} out of bounds`);
  if ((ev.overloaded || []).length) problems.push(`${ev.overloaded.length} bearing overload`);
  if ((ev.illegalStacks || []).length) problems.push(`${ev.illegalStacks.length} illegal stack`);
  if ((ev.unsupported || []).length) problems.push(`${ev.unsupported.length} unsupported`);
  const v = $('#stat-violations');
  v.textContent = problems.length ? `⚠ ${problems.join(', ')}` : '✓ valid';
  v.style.color = problems.length ? '#b00' : '#2a7';
}

// --- boot --------------------------------------------------------------------

async function boot() {
  initTabs();
  viewer = createViewer($('#view'));
  $('#case-form').addEventListener('submit', submitCase);
  $('#case-cancel').addEventListener('click', resetCaseForm);
  $('#truck-form').addEventListener('submit', submitTruck);
  $('#solve').addEventListener('click', solve);
  try {
    await refreshData();
    await solve(); // show an initial plan
  } catch (err) { setError(err.message); }
}

boot();
