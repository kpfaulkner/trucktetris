# Truck Tetris

## AIM

Exploring how to develop an efficient way of loading road cases into a truck.
Will need to take into account a number of factors:

- Size of truck/trailer
- Where axles of truck/trailer are
- Sizes of road cases
- Weights of road cases
- What types of road cases can be stacked on top of each other
- Which road cases can be laid on their side vs must stay in regular (upright) orientation
- Do some road cases (maybe due to weight) need to be over the axles?
- Do some items have to be unloaded before others? (maybe unloading at multiple sites?)

After taking the above into consideration, want to develop a website (Go backend) that allows
users to select various road cases and a truck size and maximise the cases that can go
into the truck.

Would also require the website to be able to have an interactive 3D model (purely box form) of 
the container and contents and allow manual repositioning of road cases.

Also need a data-entry page where users add/edit road case details (dimensions, weight, type,
stacking rules) and truck/trailer details (dimensions, axle positions, weight limits). This data
feeds the solver and 3D view.


## Milestones

### M1 — Domain model + Go skeleton
**Goal:** Foundations. Types + running server, no logic yet.

- Core types:
  - `Case`: id, name, dimensions (L×W×H, mm), weight (kg), type/category, list of case types
    it may be stacked on, allowed orientations (which faces may sit down — some cases can lie
    on their side, others must stay upright/regular orientation only).
  - `Truck`: id, name, internal dimensions, axle positions (distance from front), per-axle
    max load, gross max weight.
  - `Placement`: case id, position (x,y,z origin corner), rotation/orientation.
  - `LoadPlan`: truck, list of `Placement`, list of unplaced case ids, summary stats.
- Units convention: millimetres + kilograms everywhere. Documented in one place.
- HTTP server: serves static assets, `/api/health`, stub `/api/solve` echoing input.
- Hardcoded sample truck + cases for testing.

**Done when:** `go run` serves a page, `/api/solve` round-trips JSON.

### M2 — Basic packer (volume only)
**Goal:** First real solver. Geometry only, ignore weight/axles/stacking/unload.

- Solver behind an interface (`Packer`) so later versions swap in.
- Heuristic: shelf or layer-based first-fit. Sort cases largest-volume first.
- Axis-aligned only. Rotations limited to each case's allowed orientations (upright-only cases
  never laid on their side).
- AABB overlap check against already-placed cases + container bounds.
- Returns `LoadPlan` with placements + unplaced list.

**Done when:** given sample data, returns non-overlapping placements inside the truck and reports
any cases that did not fit.

### M3 — Static 3D viewer
**Goal:** See the plan. Read-only render.

- Three.js. `BoxGeometry` per placed case, `EdgesGeometry` outline for readability.
- Colour by case type, legend.
- Truck container as wireframe / semi-transparent walls.
- `OrbitControls`: orbit / pan / zoom.
- Loads `LoadPlan` JSON from `/api/solve`.
- Overlay panel: total weight, count placed / unplaced.

**Done when:** browser shows the packed truck in 3D matching solver output; pipeline
Go → JSON → render proven.

### M4 — Data-entry page
**Goal:** Real user data replaces hardcoded samples.

- Forms to add/edit/delete road cases (all `Case` fields incl. stacking rules).
- Forms to add/edit/delete trucks/trailers (dims, axle positions, weight limits).
- Persistence: store to disk (SQLite or JSON file) via Go API. CRUD endpoints.
- Validation: positive dims/weights, axle positions within truck length.
- Selection screen: pick a truck + subset of cases, send to solver.

**Done when:** user creates cases + a truck in the UI, selects them, and M2 solver runs on that
data with no code edits.

### M5 — Stacking rules + weight
**Goal:** Solver respects stack compatibility and weight.

- Enforce stackable-on rules: a case only sits on a type it is allowed to.
- Max stack height + max weight bearing down on a bottom case.
- Support check: a case needs sufficient supporting surface below (no floating boxes).
- Track running total weight; reject/flag if over truck gross max.

**Done when:** solver never stacks illegal combinations and never exceeds gross weight; viewer
shows valid stacks.

### M6 — Axle constraints
**Goal:** Legal weight distribution. Hardest solver piece.

- Compute load on each axle from placement x-positions + weights (moment / lever-arm calc).
- Enforce per-axle max load.
- Bias heavy cases toward/over axle positions during packing.
- Report axle loads in the overlay; flag overloaded axles.

**Done when:** solver output keeps every axle within limit and reports the distribution.

### M7 — Unload order
**Goal:** Multi-site loading order.

- Each case gets a drop sequence / destination.
- Constraint: earlier-drop cases must be reachable (loaded last / nearest the door / on top).
- Solver orders placement so unload sequence is physically possible without moving other cases.

**Done when:** for a multi-stop route, each stop's cases can be removed without disturbing
later-stop cases.

### M8 — Manual repositioning
**Goal:** Human override in 3D.

- Raycaster picks a box; drag to reposition (raycast onto floor/axis plane).
- Snap to grid; live AABB collision feedback (highlight red on overlap).
- Live recompute of weight + axle loads (reuse M5/M6 logic) with violation flags.
- Manual placements persist and override the solver for that plan.

**Done when:** user drags a case to a new valid spot, sees updated axle/weight readouts, and
overlaps/violations are flagged.

### M9 — Polish
**Goal:** Usable product.

- Save / load named load plans.
- Case + truck library management UI (built on M4).
- Export loading plan (printable order + positions, PDF or CSV).
- Basic packing metrics: volume utilisation %, weight utilisation %.

**Done when:** a plan can be saved, reloaded, and exported for use by loaders.

### Notes
- M1–M3 = walking skeleton on hardcoded data; value early, proves the full stack.
- Keep the solver behind the `Packer` interface — every solver milestone (M2, M5, M6, M7) is a
  swap or extension, not a rewrite.
- M8 depends on the M5/M6 recompute logic already existing.
- Weight/axle/order info is UI overlay + solver logic, not extra 3D geometry — keep the 3D to
  plain boxes.

