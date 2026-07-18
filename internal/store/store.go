// Package store persists trucks and road cases in SQLite.
//
// Complex fields (axles, stacking rules, orientations) are stored as JSON text
// columns — small, single-user data where a document-ish shape is simpler than
// extra tables.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/kenfaulkner/trucktetris/internal/domain"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// Store is a SQLite-backed persistence layer.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the database at path, runs migrations, and seeds
// sample data if the tables are empty. Use ":memory:" for an ephemeral DB.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// modernc's driver is safe for concurrent use but a single connection keeps
	// an in-memory DB from being cleared between connections.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seedIfEmpty(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS cases (
	id               TEXT PRIMARY KEY,
	name             TEXT NOT NULL,
	l                INTEGER NOT NULL,
	w                INTEGER NOT NULL,
	h                INTEGER NOT NULL,
	weight           INTEGER NOT NULL,
	type             TEXT NOT NULL,
	stackable        INTEGER NOT NULL DEFAULT 0,
	stackable_on     TEXT NOT NULL DEFAULT '[]',
	max_stack_weight INTEGER NOT NULL DEFAULT 0,
	can_lie_on_side  INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS trucks (
	id        TEXT PRIMARY KEY,
	name      TEXT NOT NULL,
	l         INTEGER NOT NULL,
	w         INTEGER NOT NULL,
	h         INTEGER NOT NULL,
	gross_max INTEGER NOT NULL,
	axles     TEXT NOT NULL DEFAULT '[]'
);`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	// Additive column migrations for databases created by earlier versions.
	for _, m := range []struct{ col, def string }{
		{"max_stack_weight", "INTEGER NOT NULL DEFAULT 0"},
		{"stackable", "INTEGER NOT NULL DEFAULT 0"},
		{"can_lie_on_side", "INTEGER NOT NULL DEFAULT 0"},
	} {
		if err := s.addColumnIfMissing("cases", m.col, m.def); err != nil {
			return err
		}
	}
	return nil
}

// addColumnIfMissing adds a column to table when it is not already present, so
// older databases pick up new fields without a destructive rebuild.
func (s *Store) addColumnIfMissing(table, column, def string) error {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("inspect %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil // already present
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, def))
	if err != nil {
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}

func (s *Store) seedIfEmpty() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM cases`).Scan(&n); err != nil {
		return fmt.Errorf("seed count: %w", err)
	}
	if n > 0 {
		return nil
	}
	for _, c := range domain.SampleCases() {
		if err := s.SaveCase(c); err != nil {
			return fmt.Errorf("seed case: %w", err)
		}
	}
	if err := s.SaveTruck(domain.SampleTruck()); err != nil {
		return fmt.Errorf("seed truck: %w", err)
	}
	return nil
}

// --- Cases -------------------------------------------------------------------

// ListCases returns all cases ordered by name.
func (s *Store) ListCases() ([]domain.Case, error) {
	rows, err := s.db.Query(`SELECT id, name, l, w, h, weight, type, stackable, stackable_on, max_stack_weight, can_lie_on_side
		FROM cases ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Case
	for rows.Next() {
		c, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCase returns one case by id, or ErrNotFound.
func (s *Store) GetCase(id string) (domain.Case, error) {
	row := s.db.QueryRow(`SELECT id, name, l, w, h, weight, type, stackable, stackable_on, max_stack_weight, can_lie_on_side
		FROM cases WHERE id = ?`, id)
	c, err := scanCase(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Case{}, ErrNotFound
	}
	return c, err
}

// SaveCase upserts a case. The case must already have an ID and be valid.
func (s *Store) SaveCase(c domain.Case) error {
	if c.ID == "" {
		return errors.New("case ID is required")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	stackableOn, _ := json.Marshal(c.StackableOn)
	_, err := s.db.Exec(`
		INSERT INTO cases (id, name, l, w, h, weight, type, stackable, stackable_on, max_stack_weight, can_lie_on_side)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, l=excluded.l, w=excluded.w, h=excluded.h,
			weight=excluded.weight, type=excluded.type, stackable=excluded.stackable,
			stackable_on=excluded.stackable_on, max_stack_weight=excluded.max_stack_weight,
			can_lie_on_side=excluded.can_lie_on_side`,
		c.ID, c.Name, c.Dim.L, c.Dim.W, c.Dim.H, c.Weight, c.Type, c.Stackable,
		string(stackableOn), c.MaxStackWeight, c.CanLieOnSide)
	return err
}

// DeleteCase removes a case by id. Missing ids return ErrNotFound.
func (s *Store) DeleteCase(id string) error {
	return s.deleteByID("cases", id)
}

// --- Trucks ------------------------------------------------------------------

// ListTrucks returns all trucks ordered by name.
func (s *Store) ListTrucks() ([]domain.Truck, error) {
	rows, err := s.db.Query(`SELECT id, name, l, w, h, gross_max, axles
		FROM trucks ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Truck
	for rows.Next() {
		t, err := scanTruck(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTruck returns one truck by id, or ErrNotFound.
func (s *Store) GetTruck(id string) (domain.Truck, error) {
	row := s.db.QueryRow(`SELECT id, name, l, w, h, gross_max, axles
		FROM trucks WHERE id = ?`, id)
	t, err := scanTruck(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Truck{}, ErrNotFound
	}
	return t, err
}

// SaveTruck upserts a truck. The truck must already have an ID and be valid.
func (s *Store) SaveTruck(t domain.Truck) error {
	if t.ID == "" {
		return errors.New("truck ID is required")
	}
	if err := t.Validate(); err != nil {
		return err
	}
	axles, _ := json.Marshal(t.Axles)
	_, err := s.db.Exec(`
		INSERT INTO trucks (id, name, l, w, h, gross_max, axles)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, l=excluded.l, w=excluded.w, h=excluded.h,
			gross_max=excluded.gross_max, axles=excluded.axles`,
		t.ID, t.Name, t.Dim.L, t.Dim.W, t.Dim.H, t.GrossMax, string(axles))
	return err
}

// DeleteTruck removes a truck by id. Missing ids return ErrNotFound.
func (s *Store) DeleteTruck(id string) error {
	return s.deleteByID("trucks", id)
}

// --- helpers -----------------------------------------------------------------

func (s *Store) deleteByID(table, id string) error {
	// table is a package-internal constant, never user input.
	res, err := s.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", table), id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanCase(sc scanner) (domain.Case, error) {
	var c domain.Case
	var stackableOn string
	if err := sc.Scan(&c.ID, &c.Name, &c.Dim.L, &c.Dim.W, &c.Dim.H, &c.Weight, &c.Type,
		&c.Stackable, &stackableOn, &c.MaxStackWeight, &c.CanLieOnSide); err != nil {
		return domain.Case{}, err
	}
	if err := json.Unmarshal([]byte(stackableOn), &c.StackableOn); err != nil {
		return domain.Case{}, fmt.Errorf("decode stackable_on for %s: %w", c.ID, err)
	}
	return c, nil
}

func scanTruck(sc scanner) (domain.Truck, error) {
	var t domain.Truck
	var axles string
	if err := sc.Scan(&t.ID, &t.Name, &t.Dim.L, &t.Dim.W, &t.Dim.H, &t.GrossMax, &axles); err != nil {
		return domain.Truck{}, err
	}
	if err := json.Unmarshal([]byte(axles), &t.Axles); err != nil {
		return domain.Truck{}, fmt.Errorf("decode axles for %s: %w", t.ID, err)
	}
	return t, nil
}
