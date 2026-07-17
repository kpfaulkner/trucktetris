# Truck Tetris

## AIM

Exploring how to develop an efficient way of loading road cases into a truck.
Will need to take into account a number of factors:

- Size of truck/trailer
- Where axles of truck/trailer are
- Sizes of road cases
- Weights of road cases
- What types of road cases can be stacked on top of each other
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
Core types: `Case` (dimensions, weight, type, stackable-on rules), `Truck` (dimensions, axle
positions, weight limits). JSON in/out. HTTP server serves static assets + `/api`. No solving
yet. Hardcoded sample data.

### M2 — Basic packer (volume only)
Ignore weight, axles, stacking, unload order. Fit boxes in container. First-fit or shelf/layer
heuristic. Returns list of `(case, position, rotation)`. Report cases that do not fit.

### M3 — Static 3D viewer
Three.js. Render truck + packed boxes from M2 output. Orbit/pan/zoom camera. Colour by type,
edge outlines. Read-only. Proves the Go -> JSON -> render pipeline.

### M4 — Stacking rules + weight
Solver respects stack compatibility and max stack weight/height. Track total weight.

### M5 — Axle constraints
Compute load per axle from box positions + weights. Enforce axle limits. Bias heavy cases over
axles. Hardest solver piece.

### M6 — Unload order
Multi-site support. Later-drop cases loaded deeper / earlier. Order becomes a packing constraint.

### M7 — Manual repositioning
Drag boxes in the 3D view. Snap to grid, collision check, live weight/axle recompute with
violation flagging. Manual overrides the solver.

### M8 — Polish
Save/load configurations, case + truck library UI, export loading plan.

### Notes
- M1–M3 = walking skeleton, value early.
- Solver complexity climbs from M2 to M6. Keep the solver swappable behind an interface.
- M7 depends on the M5/M6 recompute logic existing.

