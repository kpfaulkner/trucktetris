// 3D viewer module for Truck Tetris load plans.
//
// Coordinate mapping. The Go domain uses x = length, y = width, z = up, in mm.
// Three.js is y-up, so we map domain (x, y, z) -> three (x, z, y) and scale mm
// to metres. Boxes are positioned by centre, so we add half-size to the
// min-corner origin.

import * as THREE from 'three';
import { OrbitControls } from '/vendor/controls/OrbitControls.js';

const MM = 0.001; // mm -> m

const PALETTE = [0x4e79a7, 0xf28e2b, 0x59a14f, 0xe15759, 0xb07aa1, 0x76b7b2, 0xedc948, 0xff9da7];
const typeColours = new Map();
export function colourFor(type) {
  if (!typeColours.has(type)) {
    typeColours.set(type, PALETTE[typeColours.size % PALETTE.length]);
  }
  return typeColours.get(type);
}

// createViewer sets up a scene inside container and returns { render }.
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

  function resize() {
    const w = container.clientWidth;
    const h = container.clientHeight;
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

    return { l, w, h };
  }

  function addCase(placement, type) {
    const [px, py, pz] = placement.pos;
    const [sx, sy, sz] = placement.size;
    const l = sx * MM, wd = sy * MM, ht = sz * MM;

    const geo = new THREE.BoxGeometry(l, ht, wd);
    const mat = new THREE.MeshLambertMaterial({
      color: colourFor(type), transparent: true, opacity: 0.9,
    });
    const mesh = new THREE.Mesh(geo, mat);
    mesh.position.set((px + sx / 2) * MM, (pz + sz / 2) * MM, (py + sy / 2) * MM);
    contentGroup.add(mesh);

    const edges = new THREE.LineSegments(
      new THREE.EdgesGeometry(geo),
      new THREE.LineBasicMaterial({ color: 0x000000 }),
    );
    edges.position.copy(mesh.position);
    contentGroup.add(edges);
  }

  function frameCamera({ l, w, h }) {
    const cx = l / 2, cy = h / 2, cz = w / 2;
    const span = Math.max(l, w, h);
    camera.position.set(cx + span * 1.2, cy + span * 1.0, cz + span * 1.6);
    controls.target.set(cx, cy, cz);
    controls.update();
  }

  function render(plan, caseById) {
    clearContent();
    const dims = addTruck(plan.truck);
    for (const p of plan.placements) {
      addCase(p, caseById.get(p.caseId)?.type || 'unknown');
    }
    frameCamera(dims);
    resize();
  }

  resize();
  (function animate() {
    requestAnimationFrame(animate);
    controls.update();
    renderer.render(scene, camera);
  })();

  return { render, resize };
}
