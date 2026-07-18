// M3 read-only 3D viewer for Truck Tetris load plans.
//
// Coordinate mapping. The Go domain uses x = length, y = width, z = up, in mm.
// Three.js is y-up, so we map domain (x, y, z) -> three (x, z, y) and scale mm
// to metres for a sane camera. Boxes are positioned by their centre, so we add
// half-size to the min-corner origin.

import * as THREE from 'three';
import { OrbitControls } from '/vendor/controls/OrbitControls.js';

const MM = 0.001; // mm -> m

// Stable-ish colour per case type.
const PALETTE = [0x4e79a7, 0xf28e2b, 0x59a14f, 0xe15759, 0xb07aa1, 0x76b7b2, 0xedc948, 0xff9da7];
const typeColours = new Map();
function colourFor(type) {
  if (!typeColours.has(type)) {
    typeColours.set(type, PALETTE[typeColours.size % PALETTE.length]);
  }
  return typeColours.get(type);
}

const viewEl = document.getElementById('view');

const scene = new THREE.Scene();
scene.background = new THREE.Color(0x1a1d21);

const camera = new THREE.PerspectiveCamera(50, 1, 0.01, 1000);
const renderer = new THREE.WebGLRenderer({ antialias: true });
viewEl.appendChild(renderer.domElement);

const controls = new OrbitControls(camera, renderer.domElement);
controls.enableDamping = true;

scene.add(new THREE.AmbientLight(0xffffff, 0.7));
const dir = new THREE.DirectionalLight(0xffffff, 0.8);
dir.position.set(3, 6, 4);
scene.add(dir);

// Group holding everything for the current plan, so reloads can clear it.
let contentGroup = new THREE.Group();
scene.add(contentGroup);

function resize() {
  const w = viewEl.clientWidth;
  const h = viewEl.clientHeight;
  renderer.setSize(w, h, false);
  camera.aspect = w / h;
  camera.updateProjectionMatrix();
}
window.addEventListener('resize', resize);

function clearContent() {
  scene.remove(contentGroup);
  contentGroup.traverse((o) => {
    if (o.geometry) o.geometry.dispose();
    if (o.material) o.material.dispose();
  });
  contentGroup = new THREE.Group();
  scene.add(contentGroup);
}

// Add the truck load space as a wireframe box, min corner at the origin.
function addTruck(truck) {
  const l = truck.dim.l * MM;
  const w = truck.dim.w * MM;
  const h = truck.dim.h * MM;
  const geo = new THREE.BoxGeometry(l, h, w); // three: x=length, y=height, z=width
  const edges = new THREE.LineSegments(
    new THREE.EdgesGeometry(geo),
    new THREE.LineBasicMaterial({ color: 0x888888 }),
  );
  edges.position.set(l / 2, h / 2, w / 2);
  contentGroup.add(edges);

  // Floor grid for depth perception.
  const grid = new THREE.GridHelper(Math.max(l, w), 10, 0x555555, 0x333333);
  grid.position.set(l / 2, 0, w / 2);
  contentGroup.add(grid);

  return { l, w, h };
}

function addCase(placement, type) {
  const [px, py, pz] = placement.pos;
  const [sx, sy, sz] = placement.size;
  const l = sx * MM, wd = sy * MM, ht = sz * MM;

  const geo = new THREE.BoxGeometry(l, ht, wd); // three: x=len, y=up, z=width
  const mat = new THREE.MeshLambertMaterial({
    color: colourFor(type), transparent: true, opacity: 0.9,
  });
  const mesh = new THREE.Mesh(geo, mat);
  // three position = domain(min corner + half size) mapped x, z(up), y
  mesh.position.set((px + sx / 2) * MM, (pz + sz / 2) * MM, (py + sy / 2) * MM);
  contentGroup.add(mesh);

  const edges = new THREE.LineSegments(
    new THREE.EdgesGeometry(geo),
    new THREE.LineBasicMaterial({ color: 0x000000 }),
  );
  edges.position.copy(mesh.position);
  contentGroup.add(edges);
}

function frameCamera(dims) {
  const { l, w, h } = dims;
  const cx = l / 2, cy = h / 2, cz = w / 2;
  const span = Math.max(l, w, h);
  camera.position.set(cx + span * 1.2, cy + span * 1.0, cz + span * 1.6);
  controls.target.set(cx, cy, cz);
  controls.update();
}

function updatePanel(caseById, plan) {
  document.getElementById('truck-name').textContent = plan.truck.name || plan.truck.id;
  document.getElementById('placed').textContent = plan.summary.placedCount;
  document.getElementById('unplaced-count').textContent = plan.summary.unplacedCount;
  document.getElementById('weight').textContent = `${plan.summary.totalWeight} kg`;

  const types = new Set(plan.placements.map((p) => caseById.get(p.caseId)?.type));
  const legend = document.getElementById('legend');
  legend.innerHTML = '<b>Types</b>';
  for (const type of types) {
    const hex = colourFor(type).toString(16).padStart(6, '0');
    const row = document.createElement('div');
    row.className = 'legend-row';
    row.innerHTML = `<span class="swatch" style="background:#${hex}"></span>${type}`;
    legend.appendChild(row);
  }

  const un = document.getElementById('unplaced');
  un.textContent = plan.unplaced.length
    ? `Did not fit: ${plan.unplaced.map((id) => caseById.get(id)?.name || id).join(', ')}`
    : '';
}

async function loadAndSolve() {
  const sample = await (await fetch('/api/sample')).json();
  const caseById = new Map(sample.cases.map((c) => [c.id, c]));
  const plan = await (await fetch('/api/solve', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sample),
  })).json();

  clearContent();
  const dims = addTruck(plan.truck);
  for (const p of plan.placements) {
    addCase(p, caseById.get(p.caseId)?.type || 'unknown');
  }
  frameCamera(dims);
  updatePanel(caseById, plan);
}

document.getElementById('reload').addEventListener('click', loadAndSolve);

resize();
(function animate() {
  requestAnimationFrame(animate);
  controls.update();
  renderer.render(scene, camera);
})();

loadAndSolve().catch((err) => {
  console.error(err);
  document.getElementById('unplaced').textContent = `Error: ${err.message}`;
});
