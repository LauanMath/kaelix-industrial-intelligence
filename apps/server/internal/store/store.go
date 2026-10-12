package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("conflict")
var ErrInvalidHeartbeat = errors.New("heartbeat uptime or counters decreased")
var ErrUnauthorized = errors.New("unauthorized")

type Store struct{ pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, errors.New("invalid database configuration")
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.New("database connection failed")
	}
	s := &Store{pool: pool}
	if err = s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrations: %w", err)
	}
	return s, nil
}
func (s *Store) Close()                         { s.pool.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Transaction-level lock serializes migration runners from multiple API instances.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(684219301)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	for _, file := range files {
		version, err := strconv.Atoi(strings.SplitN(file.Name(), "_", 2)[0])
		if err != nil {
			return err
		}
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		sql, err := migrations.ReadFile("migrations/" + file.Name())
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(sql)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func dbError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23503") {
		return ErrConflict
	}
	return err
}
func NewToken() string { return base64.RawURLEncoding.EncodeToString(randBytes()) }
func randBytes() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}
func tokenHash(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }

// BootstrapAdmin only creates the first configured account; it never resets its password.
func (s *Store) BootstrapAdmin(ctx context.Context, email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO users(email,password_hash) VALUES($1,$2) ON CONFLICT(email) DO NOTHING`, email, string(hash))
	return err
}
func (s *Store) Login(ctx context.Context, email, password string) (string, time.Time, error) {
	var id int64
	var hash string
	err := s.pool.QueryRow(ctx, `SELECT id,password_hash FROM users WHERE email=$1`, email).Scan(&id, &hash)
	// A dummy hash keeps unknown accounts on the password-check path.
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", time.Time{}, err
	}
	if hash == "" {
		hash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	}
	mismatch := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if id == 0 || mismatch != nil {
		return "", time.Time{}, ErrUnauthorized
	}
	token := NewToken()
	expiry := time.Now().UTC().Add(24 * time.Hour)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`); err != nil {
		return "", time.Time{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, tokenHash(token), id, expiry); err != nil {
		return "", time.Time{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", time.Time{}, err
	}
	return token, expiry, nil
}
func (s *Store) UserAuthorized(ctx context.Context, token string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1 AND expires_at>now())`, tokenHash(token)).Scan(&ok)
	return ok, err
}
func (s *Store) Logout(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash(token))
	return err
}
func (s *Store) GatewayAuthorized(ctx context.Context, token string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT id FROM gateways WHERE token_hash=$1`, tokenHash(token)).Scan(&id)
	return id, dbError(err)
}

func NewUUID() string {
	b := randBytes()[:16]
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
