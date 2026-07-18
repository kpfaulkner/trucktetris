// 3D viewer + manual editor for Truck Tetris load plans.
//
// Coordinate mapping. The Go domain uses x = length, y = width, z = up, in mm.
// Three.js is y-up, so we map domain (x, y, z) -> three (x, z, y) and scale mm
// to metres. Boxes are positioned by centre, so we add half-size to the
// min-corner origin.

import * as THREE from 'three';
import { OrbitControls } from '/vendor/controls/OrbitControls.js';

const MM = 0.001; // mm -> m
const SNAP = 50;  // grid snap, mm
const BAD = 0xd00000; // colour for a case that collides or is out of bounds

const PALETTE = [
  0x4e79a7, 0xf28e2b, 0x59a14f, 0xe15759, 0xb07aa1, 0x76b7b2, 0xedc948, 0xff9da7,
  0x9c755f, 0x1f77b4, 0x2ca02c, 0xd62728, 0x9467bd, 0x8c564b, 0x17becf, 0xbcbd22,
];
// Colour keyed per case (by id), not per type, so different cases are visually
// distinct even when they share a type.
const caseColours = new Map();
export function colourFor(key) {
  if (!caseColours.has(key)) {
    caseColours.set(key, PALETTE[caseColours.size % PALETTE.length]);
  }
  return caseColours.get(key);
}

export function createViewer(container) {
  const scene = new THREE.Scene();
  scene.background = new THREE.Color(0x1a1d21);

  const camera = new THREE.PerspectiveCamera(50, 1, 0.01, 1000);
  const renderer = new THREE.WebGLRenderer({ antialias: true });
  container.appendChild(renderer.domElement);

  const controls = new OrbitControls(camera, renderer.domElement);
  controls.enableDamping = true;

  scene.add(new THREE.AmbientLight(0xffffff, 0.7));
  const dir = new THREE.DirectionalLight(0xffffff, 0.8);
  dir.position.set(3, 6, 4);
  scene.add(dir);

  let contentGroup = new THREE.Group();
  scene.add(contentGroup);

  // Editor state.
  let entries = [];        // { caseId, type, group, mesh, pos:[3], size:[3], up }
  let truckDim = null;     // { l, w, h } in mm
  let onChange = null;     // callback(placements) during/after a drag

  function resize() {
    const w = container.clientWidth, h = container.clientHeight;
    if (w === 0 || h === 0) return;
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
    entries = [];
  }

  function addTruck(truck) {
    const l = truck.dim.l * MM, w = truck.dim.w * MM, h = truck.dim.h * MM;
    const geo = new THREE.BoxGeometry(l, h, w);
    const edges = new THREE.LineSegments(
      new THREE.EdgesGeometry(geo),
      new THREE.LineBasicMaterial({ color: 0x888888 }),
    );
    edges.position.set(l / 2, h / 2, w / 2);
    contentGroup.add(edges);

    const grid = new THREE.GridHelper(Math.max(l, w), 10, 0x555555, 0x333333);
    grid.position.set(l / 2, 0, w / 2);
    contentGroup.add(grid);

    addAxles(truck, w, h);
    return { l, w, h };
  }

  function addAxles(truck, w, h) {
    for (const a of truck.axles || []) {
      const x = a.position * MM;
      const pts = [
        new THREE.Vector3(x, 0, 0), new THREE.Vector3(x, h, 0),
        new THREE.Vector3(x, h, w), new THREE.Vector3(x, 0, w),
        new THREE.Vector3(x, 0, 0),
      ];
      contentGroup.add(new THREE.Line(
        new THREE.BufferGeometry().setFromPoints(pts),
        new THREE.LineBasicMaterial({ color: 0xff8c00 }),
      ));
    }
  }

  // centreOf returns the three-space centre for a domain placement.
  function centreOf(pos, size) {
    return new THREE.Vector3(
      (pos[0] + size[0] / 2) * MM,
      (pos[2] + size[2] / 2) * MM,
      (pos[1] + size[1] / 2) * MM,
    );
  }

  function addCase(placement) {
    const size = placement.size;
    const geo = new THREE.BoxGeometry(size[0] * MM, size[2] * MM, size[1] * MM);
    const mesh = new THREE.Mesh(geo, new THREE.MeshLambertMaterial({
      color: colourFor(placement.caseId), transparent: true, opacity: 0.9,
    }));
    const edges = new THREE.LineSegments(
      new THREE.EdgesGeometry(geo),
      new THREE.LineBasicMaterial({ color: 0x000000 }),
    );
    const group = new THREE.Group();
    group.add(mesh, edges);
    group.position.copy(centreOf(placement.pos, size));
    contentGroup.add(group);

    const entry = {
      instanceId: placement.instanceId, caseId: placement.caseId, group, mesh,
      pos: [...placement.pos], size: [...size], up: placement.up,
    };
    mesh.userData.entry = entry;
    entries.push(entry);
  }

  function frameCamera({ l, w, h }) {
    const cx = l / 2, cy = h / 2, cz = w / 2;
    const span = Math.max(l, w, h);
    camera.position.set(cx + span * 1.2, cy + span * 1.0, cz + span * 1.6);
    controls.target.set(cx, cy, cz);
    controls.update();
  }

  function placements() {
    return entries.map((e) => ({
      instanceId: e.instanceId, caseId: e.caseId, pos: [...e.pos], size: [...e.size], up: e.up,
    }));
  }

  // applyEvaluation recolours boxes: red when colliding or out of bounds.
  function applyEvaluation(ev) {
    const bad = new Set([
      ...(ev.collisions || []), ...(ev.outOfBounds || []),
      ...(ev.unsupported || []), ...(ev.illegalStacks || []), ...(ev.overloaded || []),
    ]);
    for (const e of entries) {
      e.mesh.material.color.setHex(bad.has(e.instanceId) ? BAD : colourFor(e.caseId));
    }
  }

  function render(plan, caseById, opts = {}) {
    clearContent();
    truckDim = plan.truck.dim;
    onChange = opts.onChange || null;
    const dims = addTruck(plan.truck);
    for (const p of plan.placements) {
      addCase(p);
    }
    frameCamera(dims);
    resize();
  }

  // --- dragging --------------------------------------------------------------

  const raycaster = new THREE.Raycaster();
  const pointer = new THREE.Vector2();
  const dragPlane = new THREE.Plane();
  const planeHit = new THREE.Vector3();
  let drag = null; // { entry, offsetX, offsetZ }

  function setPointer(ev) {
    const r = renderer.domElement.getBoundingClientRect();
    pointer.x = ((ev.clientX - r.left) / r.width) * 2 - 1;
    pointer.y = -((ev.clientY - r.top) / r.height) * 2 + 1;
  }

  renderer.domElement.addEventListener('pointerdown', (ev) => {
    if (!onChange) return; // not editable
    setPointer(ev);
    raycaster.setFromCamera(pointer, camera);
    const hits = raycaster.intersectObjects(entries.map((e) => e.mesh), false);
    if (!hits.length) return;

    const entry = hits[0].object.userData.entry;
    // Horizontal plane at the box base height; drag slides it at its level.
    const baseY = entry.pos[2] * MM;
    dragPlane.set(new THREE.Vector3(0, 1, 0), -baseY);
    raycaster.ray.intersectPlane(dragPlane, planeHit);
    drag = {
      entry,
      offsetX: planeHit.x - entry.group.position.x,
      offsetZ: planeHit.z - entry.group.position.z,
    };
    controls.enabled = false;
  });

  renderer.domElement.addEventListener('pointermove', (ev) => {
    if (!drag) return;
    setPointer(ev);
    raycaster.setFromCamera(pointer, camera);
    if (!raycaster.ray.intersectPlane(dragPlane, planeHit)) return;

    const { entry } = drag;
    const cx = planeHit.x - drag.offsetX;
    const cz = planeHit.z - drag.offsetZ;

    // three centre -> domain min-corner, snap, clamp to truck bounds.
    let px = Math.round(cx / MM - entry.size[0] / 2);
    let py = Math.round(cz / MM - entry.size[1] / 2);
    px = clamp(snap(px), 0, truckDim.l - entry.size[0]);
    py = clamp(snap(py), 0, truckDim.w - entry.size[1]);
    entry.pos[0] = px;
    entry.pos[1] = py;
    // Rest on whatever is under this footprint: the highest overlapping box
    // top, or the floor. Dragging a box over another lifts it on top.
    entry.pos[2] = restingZ(entry);
    entry.group.position.copy(centreOf(entry.pos, entry.size));

    if (onChange) onChange(placements());
  });

  // restingZ returns the height (mm) the entry should sit at given its current
  // x/y footprint: the highest top of any other box it overlaps in plan view,
  // or 0 (floor) if none.
  function restingZ(entry) {
    const [px, py] = entry.pos;
    const [sx, sy] = entry.size;
    let z = 0;
    for (const o of entries) {
      if (o === entry) continue;
      const overlapXY = px < o.pos[0] + o.size[0] && o.pos[0] < px + sx &&
        py < o.pos[1] + o.size[1] && o.pos[1] < py + sy;
      if (overlapXY) z = Math.max(z, o.pos[2] + o.size[2]);
    }
    return z;
  }

  function endDrag() {
    if (!drag) return;
    drag = null;
    controls.enabled = true;
    if (onChange) onChange(placements());
  }
  renderer.domElement.addEventListener('pointerup', endDrag);
  renderer.domElement.addEventListener('pointerleave', endDrag);

  resize();
  (function animate() {
    requestAnimationFrame(animate);
    controls.update();
    renderer.render(scene, camera);
  })();

  return { render, resize, applyEvaluation, placements };
}

function snap(v) { return Math.round(v / SNAP) * SNAP; }
function clamp(v, lo, hi) { return Math.max(lo, Math.min(hi, v)); }
