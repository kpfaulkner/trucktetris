// App orchestration: tab switching, CRUD forms, selection, and solving.
import { createViewer, colourFor } from '/viewer.js';
import { buildLoadingSheet, DISCLAIMER } from '/sheet.js';

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
let planTruck = null;      // full truck object for the current plan
let planCaseById = new Map();
let planUnplaced = [];

// Current placements shown in the view (staged + positioned), and the set of
// instance IDs still sitting in the staging area (not yet placed by the user).
let manualPlacements = [];
let stagedSet = new Set();

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
    const edit = el('button', { textContent: 'Edit', className: 'small edit' });
    edit.addEventListener('click', () => startEditTruck(t));
    const del = el('button', { textContent: 'Delete', className: 'small' });
    del.addEventListener('click', async () => {
      try {
        await api('DELETE', `/api/trucks/${t.id}`);
        if (editingTruckId === t.id) resetTruckForm();
        await refreshData();
      } catch (e) { setError(e.message); }
    });
    row.append(el('td', {}, [edit, del]));
    tb.append(row);
  }
}

// --- truck edit state --------------------------------------------------------

let editingTruckId = null;

function startEditTruck(t) {
  editingTruckId = t.id;
  $('#t-name').value = t.name;
  $('#t-l').value = t.dim.l;
  $('#t-w').value = t.dim.w;
  $('#t-h').value = t.dim.h;
  $('#t-gross').value = t.grossMax;
  $('#t-heavy').value = t.heavyThreshold || 0;
  $('#t-axles').value = (t.axles || []).map((a) => `${a.position}:${a.maxLoad}`).join(', ');
  $('#truck-submit').textContent = 'Save changes';
  $('#truck-cancel').hidden = false;
  $('#t-name').focus();
}

function resetTruckForm() {
  editingTruckId = null;
  $('#truck-form').reset();
  $('#truck-submit').textContent = 'Add truck';
  $('#truck-cancel').hidden = true;
}

// Parse "pos:load, pos:load" into an axle array.
function parseAxles(text) {
  return text.split(',').map((s) => s.trim()).filter(Boolean).map((pair) => {
    const [pos, load] = pair.split(':').map((n) => +n.trim());
    return { position: pos, maxLoad: load };
  });
}

function readTruckForm() {
  return {
    name: $('#t-name').value.trim(),
    dim: { l: +$('#t-l').value, w: +$('#t-w').value, h: +$('#t-h').value },
    grossMax: +$('#t-gross').value,
    heavyThreshold: +$('#t-heavy').value,
    axles: parseAxles($('#t-axles').value),
  };
}

async function submitTruck(e) {
  e.preventDefault();
  setError('');
  try {
    if (editingTruckId) {
      await api('PUT', `/api/trucks/${editingTruckId}`, readTruckForm());
    } else {
      await api('POST', '/api/trucks', readTruckForm());
    }
    resetTruckForm();
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
    const qty = el('input', { type: 'number', min: '0', value: '1', title: 'How many to load' });
    qty.className = 'sel-qty';
    qty.dataset.id = c.id;
    qty.addEventListener('input', () => syncStaging());
    const hex = colourFor(c.id).toString(16).padStart(6, '0');
    const bears = c.stackable ? `bears ${c.maxStackWeight}kg` : 'no stacking';
    list.append(el('label', { className: 'sel-row' }, [
      qty,
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

// selectedTruck reads the current truck dropdown selection.
function selectedTruck() {
  return trucks.find((t) => t.id === $('#sel-truck').value) || null;
}

function desiredCounts() {
  const counts = {};
  for (const q of document.querySelectorAll('.sel-qty')) {
    counts[q.dataset.id] = Math.max(0, parseInt(q.value, 10) || 0);
  }
  return counts;
}

// layoutStaging places every still-staged instance in a field beside the truck
// (along its length, wrapping outward in width), so nothing overlaps and the
// user can grab and drag each into the load space.
function layoutStaging(truck) {
  const gap = 300;
  let x = 0, rowY = truck.dim.w + 800, rowDepth = 0;
  for (const p of manualPlacements) {
    if (!stagedSet.has(p.instanceId)) continue;
    if (x + p.size[0] > truck.dim.l && x > 0) {
      x = 0; rowY += rowDepth + gap; rowDepth = 0;
    }
    p.pos = [x, rowY, 0];
    x += p.size[0] + gap;
    rowDepth = Math.max(rowDepth, p.size[1]);
  }
}

// syncStaging reconciles manualPlacements with the quantity inputs: added
// instances appear staged beside the truck; removed ones drop off; already
// positioned instances keep their place. Called live as quantities change.
function syncStaging({ keepCamera = true } = {}) {
  setError('');
  const truck = selectedTruck();
  if (!truck) return;
  planTruck = truck;
  planTruckId = truck.id;
  planCaseById = new Map(cases.map((c) => [c.id, c]));

  const want = desiredCounts();
  const byCase = {};
  for (const p of manualPlacements) (byCase[p.caseId] ||= []).push(p);

  const next = [];
  for (const c of cases) {
    const have = byCase[c.id] || [];
    const n = want[c.id] || 0;
    for (let i = 0; i < n; i++) {
      if (i < have.length) {
        next.push(have[i]); // keep existing (staged or positioned)
      } else {
        const id = `${c.id}#${i}`;
        stagedSet.add(id);
        next.push({ instanceId: id, caseId: c.id, pos: [0, 0, 0], size: [c.dim.l, c.dim.w, c.dim.h], up: 'H' });
      }
    }
    // Drop instances beyond the wanted count.
    for (let i = n; i < have.length; i++) stagedSet.delete(have[i].instanceId);
  }
  manualPlacements = next;

  layoutStaging(truck);
  renderManual({ keepCamera });
  onPlacementsChanged(clonePlacements());
}

function clonePlacements() {
  return manualPlacements.map((p) => ({
    instanceId: p.instanceId, caseId: p.caseId, pos: [...p.pos], size: [...p.size], up: p.up,
  }));
}

function renderManual({ keepCamera }) {
  const plan = { truck: planTruck, placements: clonePlacements(), unplaced: [] };
  viewer.render(plan, planCaseById, { onChange: onPlacementsChanged, keepCamera });
  $('#stat-truck').textContent = planTruck.name;
  $('#stat-placed').textContent = manualPlacements.length;
  $('#stat-unplaced').textContent = 0;
}

async function solve() {
  setError('');
  const truckId = $('#sel-truck').value;
  const caseIds = [];
  for (const q of document.querySelectorAll('.sel-qty')) {
    const n = Math.max(0, parseInt(q.value, 10) || 0);
    for (let i = 0; i < n; i++) caseIds.push(q.dataset.id);
  }
  if (!truckId) { setError('Select a truck'); return; }
  if (!caseIds.length) { setError('Set a quantity for at least one case'); return; }
  try {
    const plan = await api('POST', '/api/solve', { truckId, caseIds });
    planTruckId = truckId;
    planTruck = plan.truck;
    planUnplaced = plan.unplaced || [];
    planCaseById = new Map(cases.map((c) => [c.id, c]));
    // The solver positions everything, so nothing is left staged.
    manualPlacements = (plan.placements || []).map((p) => ({
      instanceId: p.instanceId, caseId: p.caseId, pos: [...p.pos], size: [...p.size], up: p.up,
    }));
    stagedSet = new Set();
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
  // Sync app state from the view: update positions and un-stage anything the
  // user has dragged.
  const byId = new Map(manualPlacements.map((p) => [p.instanceId, p]));
  for (const pl of placements) {
    const mp = byId.get(pl.instanceId);
    if (!mp) continue;
    if (mp.pos[0] !== pl.pos[0] || mp.pos[1] !== pl.pos[1] || mp.pos[2] !== pl.pos[2]) {
      stagedSet.delete(pl.instanceId); // user moved it out of staging
    }
    mp.pos = [...pl.pos];
    mp.size = [...pl.size];
    mp.up = pl.up;
  }

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
  $('#stat-vol').textContent = `${plan.summary.volumeUtilPct}%`;
  $('#stat-wutil').textContent = `${plan.summary.weightUtilPct}%`;
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

  if (planTruck) {
    const tv = planTruck.dim.l * planTruck.dim.w * planTruck.dim.h;
    const used = viewer.placements().reduce((s, p) => s + p.size[0] * p.size[1] * p.size[2], 0);
    $('#stat-vol').textContent = tv > 0 ? `${Math.round((used / tv) * 100)}%` : '—';
    $('#stat-wutil').textContent = planTruck.grossMax > 0
      ? `${Math.round((ev.totalWeight / planTruck.grossMax) * 100)}%` : '—';
  }

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

// --- save / load / export ----------------------------------------------------

async function savePlan() {
  setError('');
  const name = $('#plan-name').value.trim();
  if (!name) { setError('Enter a plan name'); return; }
  if (!planTruckId) { setError('Solve a plan first'); return; }
  try {
    await api('POST', '/api/plans', {
      name, truckId: planTruckId,
      placements: viewer.placements(),
      unplaced: planUnplaced,
    });
    $('#plan-name').value = '';
    await renderPlanList();
  } catch (err) { setError(err.message); }
}

async function loadPlan(id) {
  setError('');
  try {
    const saved = await api('GET', `/api/plans/${id}`);
    const truck = await api('GET', `/api/trucks/${saved.truckId}`);
    planTruckId = truck.id;
    planTruck = truck;
    planUnplaced = saved.unplaced || [];
    planCaseById = new Map(cases.map((c) => [c.id, c]));

    const plan = { truck, placements: saved.placements || [], unplaced: planUnplaced };

    // Sync the "build a load" quantities to the loaded plan: per case, count is
    // placed instances plus any unplaced entries.
    const counts = {};
    for (const p of plan.placements) counts[p.caseId] = (counts[p.caseId] || 0) + 1;
    for (const id of planUnplaced) counts[id] = (counts[id] || 0) + 1;
    $('#sel-truck').value = truck.id;
    for (const q of document.querySelectorAll('.sel-qty')) {
      q.value = counts[q.dataset.id] || 0;
    }

    // Loaded placements are all positioned (nothing staged).
    manualPlacements = plan.placements.map((p) => ({
      instanceId: p.instanceId, caseId: p.caseId, pos: [...p.pos], size: [...p.size], up: p.up,
    }));
    stagedSet = new Set();

    viewer.render(plan, planCaseById, { onChange: onPlacementsChanged });
    $('#stat-truck').textContent = truck.name;
    $('#stat-placed').textContent = plan.placements.length;
    $('#stat-unplaced').textContent = planUnplaced.length;
    // Re-derive live stats (weight, axle loads, violations) from the placements.
    onPlacementsChanged(plan.placements);
  } catch (err) { setError(err.message); }
}

async function deletePlan(id) {
  try { await api('DELETE', `/api/plans/${id}`); await renderPlanList(); }
  catch (err) { setError(err.message); }
}

async function renderPlanList() {
  const list = $('#plan-list');
  try {
    const plans = await api('GET', '/api/plans');
    list.replaceChildren();
    if (!plans || !plans.length) {
      list.append(el('div', { className: 'pdate', textContent: 'No saved plans yet.' }));
      return;
    }
    for (const p of plans) {
      const load = el('button', { textContent: 'Load', className: 'small edit' });
      load.addEventListener('click', () => loadPlan(p.id));
      const del = el('button', { textContent: 'Delete', className: 'small' });
      del.addEventListener('click', () => deletePlan(p.id));
      list.append(el('div', { className: 'plan-row' }, [
        el('span', { className: 'pname', textContent: p.name }),
        el('span', { className: 'pdate', textContent: p.createdAt || '' }),
        load, del,
      ]));
    }
  } catch (err) { setError(err.message); }
}

function exportCsv() {
  const rows = [['order', 'caseId', 'name', 'weight_kg', 'x_mm', 'y_mm', 'z_mm', 'dx_mm', 'dy_mm', 'dz_mm', 'up']];
  viewer.placements().forEach((p, i) => {
    const c = planCaseById.get(p.caseId);
    rows.push([
      i + 1, p.caseId, c?.name || '', c?.weight ?? '',
      p.pos[0], p.pos[1], p.pos[2], p.size[0], p.size[1], p.size[2], p.up,
    ]);
  });
  const csv = rows.map((r) => r.map(csvCell).join(',')).join('\r\n');
  const blob = new Blob([csv], { type: 'text/csv' });
  const url = URL.createObjectURL(blob);
  const a = el('a', { href: url, download: 'loading-plan.csv' });
  document.body.append(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

// csvCell quotes a value when it contains a comma, quote, or newline.
function csvCell(v) {
  const s = String(v);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

// Open a print-friendly loading sheet in a new window, using the current
// placements and a fresh server evaluation for the compliance figures.
async function printLoadingSheet() {
  setError('');
  if (!planTruck) { setError('Solve or build a plan first'); return; }
  const placements = clonePlacements();
  if (!placements.length) { setError('Nothing to print'); return; }
  try {
    const evaluation = await api('POST', '/api/evaluate', { truckId: planTruckId, placements });
    const html = buildLoadingSheet({ truck: planTruck, placements, caseById: planCaseById, evaluation });
    const win = window.open('', '_blank');
    if (!win) { setError('Pop-up blocked — allow pop-ups to open the loading sheet'); return; }
    win.document.write(html);
    win.document.close();
  } catch (err) { setError(err.message); }
}

// --- boot --------------------------------------------------------------------

async function boot() {
  initTabs();
  $('#disclaimer').textContent = `Disclaimer: ${DISCLAIMER}`;
  viewer = createViewer($('#view'));
  $('#case-form').addEventListener('submit', submitCase);
  $('#case-cancel').addEventListener('click', resetCaseForm);
  $('#truck-form').addEventListener('submit', submitTruck);
  $('#truck-cancel').addEventListener('click', resetTruckForm);
  $('#solve').addEventListener('click', solve);
  $('#sel-truck').addEventListener('change', () => syncStaging({ keepCamera: false }));
  $('#save-plan').addEventListener('click', savePlan);
  $('#loading-sheet').addEventListener('click', printLoadingSheet);
  $('#export-csv').addEventListener('click', exportCsv);
  try {
    await refreshData();
    await renderPlanList();
    await solve(); // show an initial plan
  } catch (err) { setError(err.message); }
}

boot();
