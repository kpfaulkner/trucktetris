// Builds a print-friendly HTML loading sheet from a plan + evaluation.
//
// Translates raw mm coordinates into how a loader actually works: a numbered
// loading sequence (floor-first, front-to-back), human zones/tiers, "rests on"
// references, plus 2D top-down and side diagrams and a compliance summary.

import { colourFor } from '/viewer.js';

// Shared disclaimer text (M10) — placeholder wording, reused in-app later.
export const DISCLAIMER =
  'This load plan is a computer-generated suggestion based on the dimensions, weights, and ' +
  'rules entered. It is NOT professional advice on how to load or restrain a vehicle. It does ' +
  'not account for load restraint/tie-downs, axle-group and road-legal mass limits, dangerous ' +
  'goods, vehicle-specific limits, or site rules. The operator is responsible for verifying the ' +
  'load is safe and legal before transport.';

const esc = (s) => String(s).replace(/[&<>"]/g, (c) => (
  { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

function zoneAlong(centreX, truckL) {
  const f = centreX / truckL;
  return f < 1 / 3 ? 'Front' : f < 2 / 3 ? 'Middle' : 'Rear';
}

// Side reference: viewed from the rear doors looking toward the cab. y = 0 is
// the left-hand side under that view.
function sideAcross(centreY, truckW) {
  const f = centreY / truckW;
  return f < 1 / 3 ? 'Left' : f < 2 / 3 ? 'Centre' : 'Right';
}

function orientationWord(up) {
  return up === 'W' ? 'On side' : up === 'L' ? 'On end' : 'Upright';
}

// supporterOf returns the placement directly beneath p (top meets p's bottom,
// footprints overlap), or null if p is on the floor.
function supporterOf(p, placements) {
  for (const q of placements) {
    if (q === p) continue;
    if (q.pos[2] + q.size[2] === p.pos[2] &&
      p.pos[0] < q.pos[0] + q.size[0] && q.pos[0] < p.pos[0] + p.size[0] &&
      p.pos[1] < q.pos[1] + q.size[1] && q.pos[1] < p.pos[1] + p.size[1]) {
      return q;
    }
  }
  return null;
}

function tierOf(p, placements, memo) {
  if (memo.has(p)) return memo.get(p);
  const s = supporterOf(p, placements);
  const tier = s ? tierOf(s, placements, memo) + 1 : 1;
  memo.set(p, tier);
  return tier;
}

// buildLoadingSheet returns a complete HTML document string.
// data = { truck, placements, caseById (Map), evaluation }
export function buildLoadingSheet({ truck, placements, caseById, evaluation }) {
  // Only cases actually inside the load space are "loaded"; anything flagged
  // out of bounds is listed separately as not loaded.
  const oob = new Set(evaluation.outOfBounds || []);
  const loaded = placements.filter((p) => !oob.has(p.instanceId));
  const notLoaded = placements.filter((p) => oob.has(p.instanceId));

  // Loading order: floor tier first, then front -> back, then across.
  const order = [...loaded].sort((a, b) =>
    a.pos[2] - b.pos[2] || a.pos[0] - b.pos[0] || a.pos[1] - b.pos[1]);

  const stepOf = new Map();
  order.forEach((p, i) => stepOf.set(p.instanceId, i + 1));

  // Duplicate ordinals: "(2 of 3)".
  const totalByCase = {};
  for (const p of placements) totalByCase[p.caseId] = (totalByCase[p.caseId] || 0) + 1;

  const tierMemo = new Map();
  const nameOf = (p) => caseById.get(p.caseId)?.name || p.caseId;
  const label = (p) => {
    const m = totalByCase[p.caseId];
    if (m <= 1) return esc(nameOf(p));
    const n = parseInt(p.instanceId.split('#')[1] || '0', 10) + 1;
    return `${esc(nameOf(p))} (${n} of ${m})`;
  };

  const rows = order.map((p) => {
    const c = caseById.get(p.caseId) || {};
    const cx = p.pos[0] + p.size[0] / 2;
    const cy = p.pos[1] + p.size[1] / 2;
    const tier = tierOf(p, loaded, tierMemo);
    const sup = supporterOf(p, loaded);
    const restsOn = sup ? `on #${stepOf.get(sup.instanceId)} ${esc(nameOf(sup))}` : 'floor';
    const mFromFront = (p.pos[0] / 1000).toFixed(2);
    return `<tr>
      <td class="num">${stepOf.get(p.instanceId)}</td>
      <td>${label(p)}</td>
      <td>${zoneAlong(cx, truck.dim.l)} — ${mFromFront} m</td>
      <td>${sideAcross(cy, truck.dim.w)}</td>
      <td>Tier ${tier} — ${restsOn}</td>
      <td>${orientationWord(p.up)}</td>
      <td class="num">${c.weight ?? ''}</td>
    </tr>`;
  }).join('');

  return `<!doctype html><html lang="en"><head><meta charset="utf-8">
<title>Loading sheet — ${esc(truck.name)}</title>
<style>
  * { box-sizing: border-box; }
  body { font-family: system-ui, sans-serif; color: #1a1d21; margin: 24px; }
  h1 { font-size: 1.3rem; margin: 0 0 4px; }
  h2 { font-size: 1rem; margin: 20px 0 6px; }
  .sub { color: #555; margin: 0 0 12px; }
  table { border-collapse: collapse; width: 100%; font-size: 0.85rem; }
  th, td { border: 1px solid #ccc; padding: 4px 6px; text-align: left; vertical-align: top; }
  th { background: #f2f2f2; }
  td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
  .box { border: 1px solid #ccc; border-radius: 6px; padding: 8px 12px; margin-bottom: 12px; }
  .grid { display: flex; flex-wrap: wrap; gap: 8px 24px; }
  .grid div { font-size: 0.9rem; }
  .pass { color: #1a7f37; font-weight: 600; }
  .over { color: #b00; font-weight: 700; }
  .warn { color: #b00; }
  .diagrams { display: flex; flex-wrap: wrap; gap: 24px; }
  svg { border: 1px solid #ddd; background: #fff; }
  .disclaimer { margin-top: 20px; padding: 8px 12px; border: 1px solid #b00; border-radius: 6px;
    color: #7a0000; font-size: 0.8rem; }
  @media print { body { margin: 0; } button { display: none; } }
  .print-btn { margin-bottom: 12px; padding: 8px 16px; cursor: pointer; }
</style></head><body>
<button class="print-btn" onclick="window.print()">Print / Save as PDF</button>
<h1>Loading sheet — ${esc(truck.name)}</h1>
<p class="sub">Load space ${truck.dim.l}×${truck.dim.w}×${truck.dim.h} mm.
  Sides are described looking from the rear doors toward the cab.</p>

${complianceBox(truck, evaluation, loaded.length, notLoaded, nameOf)}

<h2>Loading sequence</h2>
<p class="sub">Load in this order: floor first, front to back, then upward.</p>
<table>
  <thead><tr>
    <th class="num">#</th><th>Case</th><th>Position (zone — from front)</th>
    <th>Side</th><th>Tier / rests on</th><th>Orientation</th><th class="num">kg</th>
  </tr></thead>
  <tbody>${rows}</tbody>
</table>

<h2>Layout</h2>
<div class="diagrams">
  <figure><figcaption>Top-down (plan) — FRONT at left</figcaption>${topDownSvg(truck, order, stepOf, caseById)}</figure>
  <figure><figcaption>Side elevation — FRONT at left</figcaption>${sideSvg(truck, order, stepOf, caseById)}</figure>
</div>

<div class="disclaimer"><strong>Disclaimer:</strong> ${esc(DISCLAIMER)}</div>
</body></html>`;
}

function complianceBox(truck, ev, loadedCount, notLoaded, nameOf) {
  const wPct = truck.grossMax > 0 ? Math.round((ev.totalWeight / truck.grossMax) * 100) : 0;
  const axles = (ev.axleLoads || []).map((a, i) =>
    `<div>Axle ${i + 1} @ ${a.position}mm: ${a.load} / ${a.maxLoad} kg
      <span class="${a.over ? 'over' : 'pass'}">${a.over ? 'OVER' : 'PASS'}</span></div>`).join('');
  const problems = [];
  if (ev.overGross) problems.push('over gross weight');
  if ((ev.overloaded || []).length) problems.push(`${ev.overloaded.length} bearing overload`);
  if ((ev.illegalStacks || []).length) problems.push(`${ev.illegalStacks.length} illegal stack`);
  if ((ev.unsupported || []).length) problems.push(`${ev.unsupported.length} unsupported`);
  if ((ev.collisions || []).length) problems.push(`${ev.collisions.length} overlapping`);

  const notLoadedLine = notLoaded.length
    ? `<div class="warn">Not loaded (outside truck): ${notLoaded.map((p) => esc(nameOf(p))).join(', ')}</div>`
    : '';
  const problemLine = problems.length
    ? `<div class="warn">Issues: ${problems.join(', ')}</div>`
    : '<div class="pass">No rule violations detected.</div>';

  return `<div class="box">
    <h2 style="margin-top:0">Summary</h2>
    <div class="grid">
      <div>Cases loaded: <strong>${loadedCount}</strong></div>
      <div>Total weight: <strong>${ev.totalWeight} kg</strong> / ${truck.grossMax} kg (${wPct}%)
        <span class="${ev.overGross ? 'over' : 'pass'}">${ev.overGross ? 'OVER' : 'PASS'}</span></div>
    </div>
    <div class="grid" style="margin-top:6px">${axles}</div>
    ${notLoadedLine}${problemLine}
  </div>`;
}

// --- SVG diagrams ------------------------------------------------------------

function scaler(truckMm, maxPx) {
  return maxPx / truckMm;
}

function topDownSvg(truck, order, stepOf, caseById) {
  const pad = 20;
  const s = scaler(truck.dim.l, 640);
  const W = truck.dim.l * s + pad * 2;
  const H = truck.dim.w * s + pad * 2 + 16;
  let cells = '';
  for (const p of order) {
    const x = pad + p.pos[0] * s;
    const y = pad + p.pos[1] * s;
    const w = p.size[0] * s;
    const h = p.size[1] * s;
    const col = colourHex(caseById.get(p.caseId)?.id || p.caseId);
    cells += `<rect x="${x}" y="${y}" width="${w}" height="${h}" fill="${col}" fill-opacity="0.5" stroke="#333"/>
      <text x="${x + w / 2}" y="${y + h / 2}" font-size="11" text-anchor="middle" dominant-baseline="central">${stepOf.get(p.instanceId)}</text>`;
  }
  return `<svg width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
    <rect x="${pad}" y="${pad}" width="${truck.dim.l * s}" height="${truck.dim.w * s}" fill="none" stroke="#000"/>
    ${cells}
    <text x="${pad}" y="${H - 4}" font-size="11" fill="#1a7f37">FRONT</text>
    <text x="${pad + truck.dim.l * s}" y="${H - 4}" font-size="11" fill="#b00" text-anchor="end">BACK</text>
  </svg>`;
}

function sideSvg(truck, order, stepOf, caseById) {
  const pad = 20;
  const s = scaler(truck.dim.l, 640);
  const W = truck.dim.l * s + pad * 2;
  const H = truck.dim.h * s + pad * 2 + 16;
  const floor = pad + truck.dim.h * s; // z=0 at the bottom of the truck box
  let cells = '';
  for (const p of order) {
    const x = pad + p.pos[0] * s;
    const w = p.size[0] * s;
    const h = p.size[2] * s;
    const y = floor - (p.pos[2] + p.size[2]) * s;
    const col = colourHex(caseById.get(p.caseId)?.id || p.caseId);
    cells += `<rect x="${x}" y="${y}" width="${w}" height="${h}" fill="${col}" fill-opacity="0.5" stroke="#333"/>
      <text x="${x + w / 2}" y="${y + h / 2}" font-size="11" text-anchor="middle" dominant-baseline="central">${stepOf.get(p.instanceId)}</text>`;
  }
  return `<svg width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">
    <rect x="${pad}" y="${pad}" width="${truck.dim.l * s}" height="${truck.dim.h * s}" fill="none" stroke="#000"/>
    ${cells}
    <text x="${pad}" y="${H - 4}" font-size="11" fill="#1a7f37">FRONT</text>
    <text x="${pad + truck.dim.l * s}" y="${H - 4}" font-size="11" fill="#b00" text-anchor="end">BACK</text>
  </svg>`;
}

// Reuse the viewer's per-case colour map so the sheet matches the 3D view.
function colourHex(key) {
  return '#' + colourFor(key).toString(16).padStart(6, '0');
}
