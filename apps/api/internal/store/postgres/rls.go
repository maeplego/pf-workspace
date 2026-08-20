package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/portfolio/pf-workspace/api/internal/store"
)

type dbHandle struct {
	pool       *pgxpool.Pool
	tenant     string
	skipTenant bool
}

func (s *Store) db() dbHandle {
	return dbHandle{pool: s.pool, tenant: s.tenant, skipTenant: s.skipTenant}
}

// WithTenant returns a store scoped to the IdP org_id (maps to workspaces.org_id).
func (s *Store) WithTenant(tenantID string) store.Store {
	cp := *s
	cp.tenant = tenantID
	cp.skipTenant = false
	return &cp
}

// Unscoped skips SET LOCAL so RLS bypass applies (migration, invite token lookup).
func (s *Store) Unscoped() store.Store {
	cp := *s
	cp.skipTenant = true
	return &cp
}

func setTenantLocal(ctx context.Context, tx pgx.Tx, tenant string, skip bool) error {
	if skip {
		return nil
	}
	_, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenant)
	return err
}

func (d dbHandle) begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	if err := setTenantLocal(ctx, tx, d.tenant, d.skipTenant); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func (d dbHandle) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tx, err := d.begin(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return tag, err
	}
	return tag, tx.Commit(ctx)
}

func (d dbHandle) queryRow(ctx context.Context, sql string, args []any, scan func(pgx.Row) error) error {
	tx, err := d.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := scan(tx.QueryRow(ctx, sql, args...)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (d dbHandle) query(ctx context.Context, sql string, args []any, fn func(pgx.Rows) error) error {
	tx, err := d.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if err := fn(rows); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type lazyRow struct {
	d    dbHandle
	ctx  context.Context
	sql  string
	args []any
}

func (d dbHandle) QueryRow(ctx context.Context, sql string, args ...any) *lazyRow {
	return &lazyRow{d: d, ctx: ctx, sql: sql, args: args}
}

func (r *lazyRow) Scan(dest ...any) error {
	return r.d.queryRow(r.ctx, r.sql, r.args, func(row pgx.Row) error {
		return row.Scan(dest...)
	})
}

type tenantRows struct {
	pgx.Rows
	tx   pgx.Tx
	ctx  context.Context
	done bool
}

func (r *tenantRows) Close() {
	if r.done {
		return
	}
	r.done = true
	if r.Rows != nil {
		r.Rows.Close()
	}
	if r.tx != nil {
		_ = r.tx.Commit(r.ctx)
	}
}

func (r *tenantRows) Commit() error {
	if r.done {
		return nil
	}
	r.done = true
	if r.Rows != nil {
		r.Rows.Close()
	}
	if r.tx == nil {
		return nil
	}
	return r.tx.Commit(r.ctx)
}

func (d dbHandle) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	tx, err := d.begin(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return &tenantRows{Rows: rows, tx: tx, ctx: ctx}, nil
}
