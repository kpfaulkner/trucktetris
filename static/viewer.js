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
  let onDragStart = null;  // callback() at the start of a drag (for undo)
  let onSelect = null;     // callback(instanceId) when a box is picked
  let caseIndex = null;    // Map caseId -> Case, for the rotated-from-natural check

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

    // FRONT (x=0, kingpin/cab end) and BACK (x=L, doors) labels on the floor.
    const front = makeLabel('FRONT', 0x33cc55);
    front.position.set(0, 0.3, w / 2);
    contentGroup.add(front);
    const back = makeLabel('BACK', 0xdd4444);
    back.position.set(l, 0.3, w / 2);
    contentGroup.add(back);

    return { l, w, h };
  }

  // makeLabel builds a camera-facing text sprite from a canvas texture.
  function makeLabel(text, colour) {
    const canvas = document.createElement('canvas');
    canvas.width = 256;
    canvas.height = 128;
    const ctx = canvas.getContext('2d');
    ctx.fillStyle = '#' + colour.toString(16).padStart(6, '0');
    ctx.font = 'bold 64px sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(text, 128, 64);
    const tex = new THREE.CanvasTexture(canvas);
    const sprite = new THREE.Sprite(new THREE.SpriteMaterial({ map: tex, transparent: true }));
    sprite.scale.set(1.2, 0.6, 1);
    return sprite;
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
    // Blue edges mark a rotated box (off its natural upright L×W pose).
    const c = caseIndex && caseIndex.get(placement.caseId);
    const rotated = c && (placement.up !== 'H' ||
      placement.size[0] !== c.dim.l || placement.size[1] !== c.dim.w);
    const edges = new THREE.LineSegments(
      new THREE.EdgesGeometry(geo),
      new THREE.LineBasicMaterial({ color: rotated ? 0x1560ff : 0x000000 }),
    );
    // Red corner dots, hidden unless the box is in a bad spot. Keeps the case's
    // own colour visible while still flagging a violation.
    const hx = size[0] * MM / 2, hy = size[2] * MM / 2, hz = size[1] * MM / 2;
    const corners = [];
    for (const sx of [-hx, hx]) for (const sy of [-hy, hy]) for (const sz of [-hz, hz]) corners.push(sx, sy, sz);
    const dotGeo = new THREE.BufferGeometry();
    dotGeo.setAttribute('position', new THREE.Float32BufferAttribute(corners, 3));
    const marker = new THREE.Points(dotGeo, new THREE.PointsMaterial({
      color: 0xff0000, size: 0.12, sizeAttenuation: true,
    }));
    marker.visible = false;

    const group = new THREE.Group();
    group.add(mesh, edges, marker);
    group.position.copy(centreOf(placement.pos, size));
    contentGroup.add(group);

    const entry = {
      instanceId: placement.instanceId, caseId: placement.caseId, group, mesh, marker,
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
      // Keep the case's own colour; flag a bad spot with red corner dots so the
      // case stays identifiable.
      e.marker.visible = bad.has(e.instanceId);
    }
  }

  // contentBounds covers the truck plus any placements outside it (staging), so
  // the camera can frame everything.
  function contentBounds(plan) {
    let l = plan.truck.dim.l, w = plan.truck.dim.w, h = plan.truck.dim.h;
    for (const p of plan.placements) {
      l = Math.max(l, p.pos[0] + p.size[0]);
      w = Math.max(w, p.pos[1] + p.size[1]);
      h = Math.max(h, p.pos[2] + p.size[2]);
    }
    return { l: l * MM, w: w * MM, h: h * MM };
  }

  function render(plan, caseById, opts = {}) {
    clearContent();
    truckDim = plan.truck.dim;
    onChange = opts.onChange || null;
    onDragStart = opts.onDragStart || null;
    onSelect = opts.onSelect || null;
    caseIndex = caseById;
    addTruck(plan.truck);
    for (const p of plan.placements) {
      addCase(p);
    }
    if (!opts.keepCamera) frameCamera(contentBounds(plan));
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
    if (onSelect) onSelect(entry.instanceId); // select for rotate etc.
    if (onDragStart) onDragStart(); // snapshot for undo before the move
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

    // three centre -> domain min-corner, snapped to grid. Not clamped to the
    // truck: boxes may be dragged outside the load space (staging / shuffling).
    // Only the origin plane is a floor (no negative coordinates).
    entry.pos[0] = Math.max(0, snap(Math.round(cx / MM - entry.size[0] / 2)));
    entry.pos[1] = Math.max(0, snap(Math.round(cz / MM - entry.size[1] / 2)));
    // Snap flush to nearby box faces / truck walls to close gaps (M13).
    snapToNeighbours(entry);
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

  // snapToNeighbours nudges the entry's x and y so a face lands flush against a
  // nearby box face or a truck wall (within SNAP_EDGE mm), reducing gaps. Only
  // boxes overlapping in the other horizontal axis and in height are considered,
  // so it snaps to things it would actually abut. The smallest nudge wins.
  function snapToNeighbours(entry) {
    const SNAP_EDGE = 150; // mm
    const wall = [truckDim.l, truckDim.w];
    for (const axis of [0, 1]) {
      const otherH = axis === 0 ? 1 : 0;
      const near = entry.pos[axis];
      const far = entry.pos[axis] + entry.size[axis];
      let best = null; // signed delta to add to pos[axis]
      const consider = (line) => {
        for (const face of [near, far]) {
          const d = line - face;
          if (Math.abs(d) <= SNAP_EDGE && (best === null || Math.abs(d) < Math.abs(best))) best = d;
        }
      };
      consider(0);
      consider(wall[axis]);
      for (const o of entries) {
        if (o === entry) continue;
        const overlaps = (a) =>
          entry.pos[a] < o.pos[a] + o.size[a] && o.pos[a] < entry.pos[a] + entry.size[a];
        if (!overlaps(otherH) || !overlaps(2)) continue;
        consider(o.pos[axis]);
        consider(o.pos[axis] + o.size[axis]);
      }
      if (best !== null) entry.pos[axis] = Math.max(0, entry.pos[axis] + best);
    }
  }

  // dropZ returns how far the entry can fall: the highest top of a box below it
  // that its footprint overlaps, or the floor (0).
  function dropZ(entry) {
    let z = 0;
    for (const o of entries) {
      if (o === entry) continue;
      const top = o.pos[2] + o.size[2];
      const overlapXY = entry.pos[0] < o.pos[0] + o.size[0] && o.pos[0] < entry.pos[0] + entry.size[0] &&
        entry.pos[1] < o.pos[1] + o.size[1] && o.pos[1] < entry.pos[1] + entry.size[1];
      if (overlapXY && top <= entry.pos[2]) z = Math.max(z, top);
    }
    return z;
  }

  // settleAll applies gravity: every box falls until it rests on the floor or a
  // box beneath it, so moving a support never leaves another box floating.
  function settleAll() {
    for (let guard = 0; guard <= entries.length + 1; guard++) {
      let moved = false;
      for (const e of [...entries].sort((a, b) => a.pos[2] - b.pos[2])) {
        const z = dropZ(e);
        if (z < e.pos[2]) { e.pos[2] = z; moved = true; }
      }
      if (!moved) break;
    }
    for (const e of entries) e.group.position.copy(centreOf(e.pos, e.size));
  }

  function endDrag() {
    if (!drag) return;
    drag = null;
    controls.enabled = true;
    settleAll(); // no floating boxes: drop anything that lost its support
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

  return { render, resize, applyEvaluation, placements, settle: () => { settleAll(); return placements(); } };
}

function snap(v) { return Math.round(v / SNAP) * SNAP; }
