package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func New(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS admin_users (
			id         BIGSERIAL    PRIMARY KEY,
			username   VARCHAR(255) UNIQUE NOT NULL,
			password   VARCHAR(255) NOT NULL,
			created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate admin_users: %w", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS groups (
			id           BIGSERIAL    PRIMARY KEY,
			name         VARCHAR(255) UNIQUE NOT NULL,
			description  TEXT         NOT NULL DEFAULT '',
			cluster_role VARCHAR(255) NOT NULL DEFAULT '',
			custom_role  BOOLEAN      NOT NULL DEFAULT FALSE,
			rules        JSONB        NOT NULL DEFAULT '[]',
			ns_bindings  JSONB        NOT NULL DEFAULT '[]',
			created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate groups: %w", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS app_settings (
			key   VARCHAR(255) PRIMARY KEY,
			value TEXT         NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate app_settings: %w", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id              BIGSERIAL    PRIMARY KEY,
			name            VARCHAR(255) UNIQUE NOT NULL,
			groups          TEXT[]       NOT NULL DEFAULT '{}',
			cluster_role    VARCHAR(255) NOT NULL DEFAULT '',
			custom_role     BOOLEAN      NOT NULL DEFAULT FALSE,
			rules           JSONB        NOT NULL DEFAULT '[]',
			ns_bindings     JSONB        NOT NULL DEFAULT '[]',
			cert_pem        TEXT         NOT NULL DEFAULT '',
			private_key_pem TEXT         NOT NULL DEFAULT '',
			created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrate users: %w", err)
	}
	return nil
}

// SeedAdmin creates the initial admin user if the table is empty.
func SeedAdmin(ctx context.Context, pool *pgxpool.Pool, username, passwordHash string) error {
	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM admin_users").Scan(&count); err != nil {
		return fmt.Errorf("count admin_users: %w", err)
	}
	if count > 0 {
		return nil
	}
	_, err := pool.Exec(ctx,
		"INSERT INTO admin_users (username, password) VALUES ($1, $2)",
		username, passwordHash,
	)
	if err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	return nil
}
