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
  - `Placement`: instance id (unique per placed box), case id, position (x,y,z origin corner),
    world-aligned size (dx,dy,dz), up-axis. (Instance id added in M7 for loading duplicates.)
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
- Colour **per case** (keyed by case id) from a fixed palette, so different cases are visually
  distinct even when they share a type. (Originally coloured by type, but two cases of the same
  type — e.g. the cable and lighting trunks — came out identical; switched to per-case.) The same
  colour map is shared with the selection-list swatches for consistency.
- Truck container as wireframe / semi-transparent walls.
- **FRONT** (green, x=0, cab/kingpin end) and **BACK** (red, x=L, doors) labels on the floor —
  camera-facing text sprites from a canvas texture — so the truck's orientation is unambiguous
  (x runs front→back, matching the axle positions).
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

Inline editing (both tables): an **Edit** button per row loads that record into its form, which
switches to update mode (submit becomes "Save changes", a Cancel button appears) and PUTs to
`/api/{cases,trucks}/{id}`; deleting the record currently being edited resets the form. Cases
gained this alongside M7; trucks/trailers have the same edit flow. Server-side validation still
applies (e.g. an axle beyond the truck length is rejected with 400).

Seeded trucks (placeholders for an empty DB; real data is set by the operator via the UI —
see the case-data-authority note):

- `Sample 12t rigid` — 7.2 × 2.4 × 2.4 m, two axles.
- `Semi-trailer (tautliner, 13.6m)` — standard tautliner load space **13.6 × 2.4 × 2.7 m**,
  ~24 t payload. Axles measured from the front of the load space (kingpin end): prime-mover
  drive group @1600 mm (max 15 t), trailer tri-axle group @10500 mm (max 20 t). `heavyThreshold`
  left at 0 for the operator to set. Added to already-seeded databases via the trucks API (seed
  only runs on an empty DB).

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

- Request `{truckId, placements}`; the server looks up each referenced case from the store (for
  weight + stacking rules), then returns `Evaluation{axleLoads, totalWeight, overGross,
  collisions[], outOfBounds[], unsupported[], illegalStacks[], overloaded[]}`.
- Reuses the M6 axle-load model; AABB overlap check (touching faces do **not** count), a bounds
  check, and — added after a bug where manual stacks bypassed the solver's rules — the full
  stacking checks: support (no floating box), stack legality (`stackable` + `stackableOn` type),
  and bearing capacity down the support chain. Keeps all rules server-side — the browser only
  renders feedback.
- Violation lists are keyed by placement **instance ID** (see below), not case ID, so duplicate
  cases are flagged independently.

Editor (`static/viewer.js`, `static/app.js`):

- Each case is a Three.js `Group` (mesh + edge outline). A raycaster picks the box under the
  cursor.
- Drag slides the box on a **horizontal plane at its current height** (length × width). Orbit
  controls are disabled mid-drag.
- Position **snaps to a 50 mm grid**. It is **not** clamped to the truck: a box can be dragged
  out of the load space (into the staging area or off to the side) to shuffle things around; the
  only limit is no negative coordinates. A box parked outside the truck flags red ("out of
  bounds" = not loaded) and clears once dragged back in.
- On drag (throttled to one in-flight request; a drag that lands mid-request re-runs once it
  returns) the client calls `/api/evaluate` and updates live: total weight (⚠ over gross),
  per-axle loads, and a `✓ valid` / `⚠ …` violations line. A box in a bad spot (collision,
  out-of-bounds, unsupported, illegal stack, or bearing overload) is flagged with **red corner
  dots** — the case keeps its own colour so its type stays identifiable (an earlier version
  filled the whole box red, which lost the colour coding).
- **Solve & render** re-runs the solver and discards manual edits (the solver output overrides).

**Done when:** user drags a case to a new valid spot, sees updated axle/weight readouts, and
overlaps/violations are flagged. ✅

Vertical stacking by drag: while dragging, the box **auto-rests** on whatever is under its
footprint — the highest overlapping box top, or the floor if none. Dragging a box over another
lifts it on top; dragging it back to clear floor drops it down. No separate vertical control is
needed. (A drag that leaves a box overhanging or interpenetrating is flagged red by the
evaluation, same as any other violation.)

Quantity + placement identity:

- The "build a load" selection takes a **quantity** per case (0 = skip), so the same case can be
  loaded multiple times.
- Each placement carries a unique `instanceId` (`caseId#n`) as its identity; `caseId` stays the
  definition reference (dimensions, weight, rules). The solver assigns instance IDs before
  packing.
- Everything that acts on an individual box — collision/violation flagging, drag selection,
  recolouring, CSV rows — keys off `instanceId`, so dragging or flagging one copy never affects
  its identical twin.

Live staging (hand-loading workflow):

- Changing a case's quantity **instantly** adds/removes boxes in the 3D view. New instances appear
  **staged beside the truck** — laid out in a non-overlapping field along the truck length,
  wrapping outward in width — so the user can drag each into the load space by hand instead of
  running the solver.
- Staged boxes sit outside the load space, so they read as red "out of bounds" until dragged in
  (i.e. "not loaded yet"). Dragging one in un-stages it; the live evaluation updates.
- Already-positioned boxes keep their place when quantities change; lowering a quantity removes
  the surplus, raising it adds more staged boxes. Switching trucks re-stages beside the new truck.
- Unified client state: a single `manualPlacements` list plus a `stagedSet` of not-yet-placed
  instance IDs is the source of truth for the view, evaluation, save, and export. **Solve &
  render** repacks everything (clearing staging); **Load** restores a saved plan (all positioned).
- Viewer support: a `keepCamera` option avoids resetting the camera on incremental staging, and
  the camera frames all content (truck + staging area), not just the truck.

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
- Loading also **syncs the "build a load" controls** back to the saved plan: the truck dropdown is
  set to the plan's truck, and each case's quantity is set to its count in the plan (placed
  instances + unplaced entries; 0 for cases not in it). So the selection reflects what was loaded
  and can be re-solved from there.
- Plan panel: name field + Save, and a list of saved plans with Load / Delete.

Export:

- Client-side **CSV** download of the current plan: order, case id, name, weight, position
  (x/y/z mm), size (dx/dy/dz mm), up-axis. Proper CSV quoting. (CSV, not PDF — no external
  library; opens in any spreadsheet and prints from there.)

Case + truck library management: already delivered in M4 (Manage tab); M7 added inline case
editing.

**Done when:** a plan can be saved, reloaded, and exported for use by loaders. ✅

### M9 — Loading paperwork
**Goal:** Turn a plan into paperwork a loader can follow at the truck, without reading raw
coordinates. The plan already knows *where* everything goes; the sheet must explain *how to load
it* in human terms.

**Delivery:** a print-friendly **HTML "loading sheet"** (opens in the browser, Save-as-PDF /
print). SVG diagrams print crisply and need no PDF library — matches the existing stack. Can be
swapped for a generated PDF later if required.

**Coordinate → human translation** (the key idea — loaders think in order, zones, tiers, and
"what sits on what", never in mm):

- **Along length** → metres from the front **plus a zone word**: Front / Middle / Rear (length
  split in thirds).
- **Across width** → **Left / Centre / Right** (thirds), with the reference stated explicitly:
  *viewed from the rear doors looking toward the cab*.
- **Height** → **Tier** number (Tier 1 = floor, Tier 2 = on top, …) plus an explicit
  **"rests on"**: the floor, or the step number + case it sits on.
- **Orientation** → words, not axes: Upright / On side / On end.

**Loading sequence (the spine of the sheet).** A numbered, ordered step list — this is what makes
the plan physically loadable. Order = **floor tier first, then front → back, then upward**, so a
base is always placed before anything stacked on it and the front is filled before the rear
blocks access (rear-door loading). Each step row:

| Step | Case (n of m if duplicated) | Zone + m from front | Side | Tier / rests on | Orientation | Weight |

**2D diagrams** (more useful on paper than the 3D view; SVG):

- **Top-down plan** — truck outline with FRONT/BACK marked, each footprint drawn to scale,
  labelled with its step number and case colour. Shows the left/right + front/back layout at a
  glance.
- **Side elevation** — view along the length showing tiers / stacking heights, step-numbered, so
  "what is stacked on what" is obvious.

**Compliance summary box** (what a loader/driver signs off on): total weight vs `grossMax` (with
%), **per-axle load vs limit with PASS / OVER**, case count placed, any unplaced cases called
out, and volume/weight utilisation.

**Notes / warnings:** heavy-over-axle reminder, upright-only / fragile cases flagged, and any
live violations (overload, unsupported, illegal stack) surfaced prominently.

**Foot:** the M10 disclaimer (suggestion only, not professional load advice).

**Done when:** from a solved or hand-edited plan, the user prints a sheet whose numbered sequence
+ diagrams let someone load the truck correctly without opening the app.

### M10 — Disclaimer
**Goal:** Make clear the tool gives a *suggestion*, not certified load advice — protects the
user and sets honest expectations, since real loading involves restraint, load rating, road
rules, and judgement the tool does not model.

Where it appears:

- **Loading sheet (M9)** — a disclaimer block at the foot of every printed/exported sheet, so it
  travels with the paperwork that reaches the loader.
- **App** — a short line in the plan panel / near the export button, and once on first use (or in
  an "About") so it is seen in-app too.

Wording (plain, non-legalese; final text is the user's call — placeholder):

> This load plan is a computer-generated **suggestion** based on the dimensions, weights, and
> rules entered. It is **not** professional advice on how to load or restrain a vehicle. It does
> not account for load restraint/tie-downs, axle-group and road-legal mass limits, dangerous
> goods, vehicle-specific limits, or site rules. The operator is responsible for verifying the
> load is safe and legal before transport.

What it should reference (so it is not hand-wavy):

- Restraint / securing is **not** modelled (the tool only checks geometry, stacking, bearing,
  gross + per-axle mass via a simplified lever-rule estimate).
- Axle loads are an **estimate**, not a certified weighbridge figure.
- Compliance with local road/transport regulations remains the operator's responsibility.

Implementation: keep the text in one place (a constant / small template) reused by the sheet and
the in-app notice, so it stays consistent and is easy to update.

**Done when:** the disclaimer appears on every exported/printed loading sheet and is visible in
the app, sourced from a single shared string.

### Notes
- M1–M3 = walking skeleton on hardcoded data; value early, proves the full stack.
- Keep the solver behind the `Packer` interface — every solver milestone (M2, M5, M6) is a
  swap or extension, not a rewrite.
- M7 depends on the M5/M6 recompute logic already existing.
- Weight/axle info is UI overlay + solver logic, not extra 3D geometry — keep the 3D to
  plain boxes.
- Single unload site only — unload order / multi-drop sequencing is out of scope by design.

