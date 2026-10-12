package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	_ "modernc.org/sqlite"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Profile struct {
	Name       string `json:"name"`
	Email      string `json:"email"`
	Company    string `json:"company"`
	JobTitle   string `json:"jobTitle"`
	AvatarData string `json:"avatarData"`
	UpdatedAt  string `json:"updatedAt"`
}
type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	file.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(e error) (*Store, error) { db.Close(); return nil, e }
	if _, err = db.Exec("PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;"); err != nil {
		return fail(err)
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if version > 3 {
		return fail(errors.New("versão do banco não suportada"))
	}
	if version == 0 {
		script, _ := migrations.ReadFile("migrations/001_profile.sql")
		tx, e := db.Begin()
		if e != nil {
			return fail(e)
		}
		if _, e = tx.Exec(string(script)); e != nil {
			tx.Rollback()
			return fail(e)
		}
		if e = tx.Commit(); e != nil {
			return fail(e)
		}
	}
	if version < 2 {
		if _, err = db.Exec("ALTER TABLE profile ADD COLUMN avatar_data TEXT NOT NULL DEFAULT ''; PRAGMA user_version=2;"); err != nil {
			return fail(err)
		}
	}
	if version < 3 {
		script, e := migrations.ReadFile("migrations/003_api_connection.sql")
		if e != nil {
			return fail(e)
		}
		tx, e := db.Begin()
		if e != nil {
			return fail(e)
		}
		if _, e = tx.Exec(string(script)); e != nil {
			tx.Rollback()
			return fail(e)
		}
		if e = tx.Commit(); e != nil {
			return fail(e)
		}
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Profile(ctx context.Context) (Profile, error) {
	var p Profile
	err := s.db.QueryRowContext(ctx, "SELECT name,email,company,job_title,updated_at,avatar_data FROM profile WHERE id=1").Scan(&p.Name, &p.Email, &p.Company, &p.JobTitle, &p.UpdatedAt, &p.AvatarData)
	return p, err
}
func (s *Store) SaveProfile(ctx context.Context, p Profile) (Profile, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Email = strings.TrimSpace(p.Email)
	p.Company = strings.TrimSpace(p.Company)
	p.JobTitle = strings.TrimSpace(p.JobTitle)
	if p.Name == "" || utf8.RuneCountInString(p.Name) > 100 {
		return Profile{}, errors.New("informe um nome com até 100 caracteres")
	}
	if len(p.Email) > 254 {
		return Profile{}, errors.New("e-mail muito longo")
	}
	if p.Email != "" {
		address, err := mail.ParseAddress(p.Email)
		if err != nil || address.Address != p.Email {
			return Profile{}, errors.New("e-mail inválido")
		}
	}
	if utf8.RuneCountInString(p.Company) > 120 || utf8.RuneCountInString(p.JobTitle) > 100 {
		return Profile{}, errors.New("empresa ou cargo excede o limite de caracteres")
	}
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, "UPDATE profile SET name=?,email=?,company=?,job_title=?,updated_at=? WHERE id=1", p.Name, p.Email, p.Company, p.JobTitle, p.UpdatedAt)
	return p, err
}

func (s *Store) SaveAvatar(ctx context.Context, data string) (Profile, error) {
	_, err := s.db.ExecContext(ctx, "UPDATE profile SET avatar_data=?,updated_at=? WHERE id=1", data, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return Profile{}, err
	}
	return s.Profile(ctx)
}

// APISession stays in the private SQLite store. Passwords are never persisted.
type APISession struct{ Endpoint, Email, Token, ExpiresAt string }

func (s *Store) APISession(ctx context.Context) (APISession, error) {
	var v APISession
	err := s.db.QueryRowContext(ctx, `SELECT endpoint,email,token,expires_at FROM api_connection WHERE id=1`).Scan(&v.Endpoint, &v.Email, &v.Token, &v.ExpiresAt)
	return v, err
}
func (s *Store) SaveAPISession(ctx context.Context, v APISession) error {
	_, err := s.db.ExecContext(ctx, `UPDATE api_connection SET endpoint=?,email=?,token=?,expires_at=? WHERE id=1`, v.Endpoint, v.Email, v.Token, v.ExpiresAt)
	return err
}
