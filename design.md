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
  - `Case`: id, name, dimensions (L×W×H, mm), weight (kg), type/category, plus the stacking and
    orientation flags (see M5 for the finalised model): `stackable` (may bear load on top),
    `stackableOn` (which types it may sit on), `maxStackWeight` (kg it can bear, when stackable),
    `canLieOnSide` (may be rotated off upright).
  - `Truck`: id, name, internal dimensions, axle positions (distance from front), per-axle
    max load, gross max weight.
  - `Placement`: case id, position (x,y,z origin corner), world-aligned size (dx,dy,dz), up-axis.
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

### M5 — Stacking rules + weight  ✅ implemented
**Goal:** Solver respects stack compatibility and weight.

Stacking is **not** weight-only. Each `Case` carries explicit flags, and a case may be placed on
top of a supporting case only when **all** of these hold:

1. `stackable` — the supporting case is flagged to bear load at all. A `false` here blocks
   stacking regardless of weight or type (e.g. a case with a sloped/fragile top).
2. `stackableOn` — the supporting case's `type` is in the top case's allowed list (type-level
   compatibility, layered on top of the `stackable` gate).
3. `maxStackWeight` — the supporting case, and every case below it in the support chain, stays
   within its bearing capacity (kg). Only meaningful when `stackable` is true.

Orientation is a separate flag:

- `canLieOnSide` — the upright orientation is always allowed; when true the packer may also lay
  the case on its side or end. (Replaced the earlier per-axis `uprightAxes` model — simpler UI,
  a single checkbox.)

Solver (`packer.Stacker`, extreme-point heuristic):

- Cases sorted heaviest-first (keeps heavy cases low; helps M6 axle work).
- Support model: a stacked case rests **entirely** on the top face of exactly one supporting case
  (no bridging across multiple boxes); otherwise it must sit on the floor. This is the "no
  floating boxes" support check.
- Weight bearing propagates down the whole support chain, not just the immediate parent.
- Tracks running total weight; skips any case that would exceed the truck's `grossMax`.

Persistence: `stackable`, `stackable_on`, `max_stack_weight`, `can_lie_on_side` columns, with
additive `ALTER TABLE` migrations so pre-M5 databases upgrade in place.

**Done when:** solver never stacks illegal combinations and never exceeds gross weight; viewer
shows valid stacks. ✅

**Not yet modelled:** max stack *height* as a separate per-case limit — currently bounded only by
the truck's internal height. Revisit if real cases need a tighter cap.

### M6 — Axle constraints  ✅ implemented
**Goal:** Legal weight distribution. Hardest solver piece.

Load distribution model (`domain.ComputeAxleLoads`, reusable — also feeds M8 recompute):

- Each placed case contributes a `PointLoad` = its weight acting at the centre of its footprint
  along the truck length (`pos.x + size.x/2`).
- Each point load is split between the two axles that bracket it, by the **lever rule** — the
  closer axle takes the larger share (load at the midpoint splits 50/50; load directly over an
  axle goes entirely to it).
- A load ahead of the first axle or behind the last is assigned entirely to that nearest axle —
  overhang is clamped, so no negative reactions.
- Single axle carries everything; N axles supported (the bracketing pair is found from axles
  sorted by position). This is a practical heuristic, not full statics (3+ axles are
  statically indeterminate — not solved exactly).

Solver (`packer.Stacker`):

- **Enforcement:** a candidate placement is rejected if adding the case there would push any axle
  over its `maxLoad` (`axleFeasible` recomputes the full distribution per candidate). A case that
  cannot be placed anywhere without overloading an axle is left unplaced.
- **Bias:** candidate positions are tie-broken by distance to the nearest axle (after the
  lowest-z rule), so heavy cases — packed first — settle over the axles. No-op for trucks with no
  axles.

Reporting:

- `LoadPlan.AxleLoads` — one `AxleLoad{position, load, maxLoad, over}` per axle.
- Plan panel lists per-axle `load / maxLoad`; overloaded axles flagged red with a warning.
- 3D view marks each axle with an orange hoop across the truck cross-section (plus a floor line)
  at its position, so axle locations are visible.

**Done when:** solver output keeps every axle within limit and reports the distribution. ✅

**Refinement — absolute weight threshold ✅ implemented:** the axle bias keys off an absolute
weight, not relative heaviness. `Truck.HeavyThreshold` (kg); a case is biased over the axles only
when its weight ≥ the threshold (and the threshold > 0). Lighter cases ignore the bias and pack
for density. Per-axle max load is still enforced for every case. Threshold 0 disables the bias.

### M7 — Manual repositioning  ✅ implemented
**Goal:** Human override in 3D.

Evaluation stays in Go and is reused by the editor (`domain.EvaluatePlan`, `POST /api/evaluate`):

- Request `{truckId, placements}`; the server looks up each case's weight from the store, then
  returns `Evaluation{axleLoads, totalWeight, overGross, collisions[], outOfBounds[]}`.
- Reuses the M6 axle-load model; AABB overlap check (touching faces do **not** count) and a
  bounds check against the load space. Keeps all rules server-side — the browser only renders
  feedback.

Editor (`static/viewer.js`, `static/app.js`):

- Each case is a Three.js `Group` (mesh + edge outline). A raycaster picks the box under the
  cursor.
- Drag slides the box on a **horizontal plane at its current height** (length × width). Orbit
  controls are disabled mid-drag.
- Position **snaps to a 50 mm grid** and is **clamped** to the truck bounds.
- On drag (throttled to one in-flight request; a drag that lands mid-request re-runs once it
  returns) the client calls `/api/evaluate` and updates live: total weight (⚠ over gross),
  per-axle loads, and a `✓ valid` / `⚠ …` violations line. Colliding / out-of-bounds boxes are
  recoloured **red** in the view.
- **Solve & render** re-runs the solver and discards manual edits (the solver output overrides).

**Done when:** user drags a case to a new valid spot, sees updated axle/weight readouts, and
overlaps/violations are flagged. ✅

Vertical stacking by drag: while dragging, the box **auto-rests** on whatever is under its
footprint — the highest overlapping box top, or the floor if none. Dragging a box over another
lifts it on top; dragging it back to clear floor drops it down. No separate vertical control is
needed. (A drag that leaves a box overhanging or interpenetrating is flagged red by the
evaluation, same as any other violation.)

**Scope notes:**
- Manual placements persist **in-session** (client state; survive tab switches, cleared on
  re-solve). Saving an edited plan to the database is deferred to M8.

### M8 — Polish  ✅ implemented
**Goal:** Usable product.

Packing metrics (`domain.Summary`):

- `volumeUtilPct` — placed-case volume as a percentage of the truck load-space volume.
- `weightUtilPct` — total placed weight as a percentage of `grossMax`.
- Computed by `Stacker` at solve time; shown in the plan panel and recomputed client-side during
  manual edits (volume from placements, weight from the live evaluation).

Save / load named plans:

- `domain.SavedPlan{id, name, truckId, placements, unplaced, createdAt}`; `saved_plans` table
  (placements/unplaced stored as JSON, `created_at` defaulted by SQLite).
- Store CRUD: `ListPlans` (metadata only, newest first), `GetPlan`, `SavePlan` (upsert),
  `DeletePlan`. Endpoints `GET/POST /api/plans`, `GET/DELETE /api/plans/{id}`.
- A save captures the **current** placements, including manual edits from M7. Loading fetches the
  plan + its truck, rebuilds the 3D view (editable), and re-derives live stats via
  `/api/evaluate`.
- Plan panel: name field + Save, and a list of saved plans with Load / Delete.

Export:

- Client-side **CSV** download of the current plan: order, case id, name, weight, position
  (x/y/z mm), size (dx/dy/dz mm), up-axis. Proper CSV quoting. (CSV, not PDF — no external
  library; opens in any spreadsheet and prints from there.)

Case + truck library management: already delivered in M4 (Manage tab); M7 added inline case
editing.

**Done when:** a plan can be saved, reloaded, and exported for use by loaders. ✅

### Notes
- M1–M3 = walking skeleton on hardcoded data; value early, proves the full stack.
- Keep the solver behind the `Packer` interface — every solver milestone (M2, M5, M6) is a
  swap or extension, not a rewrite.
- M7 depends on the M5/M6 recompute logic already existing.
- Weight/axle info is UI overlay + solver logic, not extra 3D geometry — keep the 3D to
  plain boxes.
- Single unload site only — unload order / multi-drop sequencing is out of scope by design.

