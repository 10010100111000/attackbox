package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists      = errors.New("username already exists")
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPass     = errors.New("invalid password")
	ErrUserDisabled    = errors.New("user account is disabled")
	ErrInvalidUsername = errors.New("username must be 3-32 characters and contain only letters, numbers, underscores or hyphens")
	usernameRegex      = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // "admin" or "user"
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
}

func NewStore(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		active INTEGER NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS box_leases (
		username TEXT PRIMARY KEY,
		box_id TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		role TEXT NOT NULL,
		expires_at DATETIME NOT NULL
	);
	`
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	// 保证默认管理员账号存在
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count); err != nil {
		return fmt.Errorf("check admin exists: %w", err)
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte("qwer1234!"), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash default admin password: %w", err)
		}
		if _, err := s.db.Exec("INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, 'admin', 1)", "admin", string(hash)); err != nil {
			return fmt.Errorf("insert default admin: %w", err)
		}
	}

	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Authenticate(username, password string) (*User, error) {
	var u User
	var activeInt int
	query := "SELECT id, username, password_hash, role, active, created_at FROM users WHERE username = ?"
	err := s.db.QueryRow(query, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &activeInt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	u.Active = (activeInt == 1)
	if !u.Active {
		return nil, ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPass
	}

	return &u, nil
}

func (s *Store) Register(username, password string) (*User, error) {
	return s.CreateUser(username, password, "user")
}

func (s *Store) CreateUser(username, password, role string) (*User, error) {
	if !usernameRegex.MatchString(username) {
		return nil, ErrInvalidUsername
	}
	if password == "" {
		return nil, errors.New("password cannot be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	query := "INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, ?, 1)"
	res, err := s.db.Exec(query, username, string(hash), role)
	if err != nil {
		return nil, ErrUserExists
	}

	id, _ := res.LastInsertId()
	return &User{
		ID:        id,
		Username:  username,
		Role:      role,
		Active:    true,
		CreatedAt: time.Now(),
	}, nil
}

func (s *Store) ListUsers() ([]User, error) {
	query := "SELECT id, username, role, active, created_at FROM users ORDER BY id ASC"
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []User
	for rows.Next() {
		var u User
		var activeInt int
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &activeInt, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Active = (activeInt == 1)
		list = append(list, u)
	}
	return list, nil
}

func (s *Store) UpdatePassword(username, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	res, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", string(hash), username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Store) ToggleActive(username string) (bool, error) {
	var activeInt int
	err := s.db.QueryRow("SELECT active FROM users WHERE username = ?", username).Scan(&activeInt)
	if err != nil {
		return false, err
	}
	newActive := 0
	if activeInt == 0 {
		newActive = 1
	}
	_, err = s.db.Exec("UPDATE users SET active = ? WHERE username = ?", newActive, username)
	return newActive == 1, err
}

func (s *Store) SetRole(username, newRole string) error {
	res, err := s.db.Exec("UPDATE users SET role = ? WHERE username = ?", newRole, username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Store) DeleteUser(username string) error {
	res, err := s.db.Exec("DELETE FROM users WHERE username = ?", username)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *Store) CountActiveAdmins() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' AND active = 1").Scan(&count)
	return count, err
}

func (s *Store) GetUser(username string) (*User, error) {
	var u User
	var activeInt int
	query := "SELECT id, username, role, active, created_at FROM users WHERE username = ?"
	err := s.db.QueryRow(query, username).Scan(&u.ID, &u.Username, &u.Role, &activeInt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	u.Active = (activeInt == 1)
	return &u, nil
}

// BoxLeaseRecord 表示持久化在 SQLite 中的沙箱租约
type BoxLeaseRecord struct {
	Username  string    `json:"username"`
	BoxID     string    `json:"box_id"`
	StartedAt time.Time `json:"started_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Store) SaveBoxLease(username, boxID string, startedAt, expiresAt time.Time) error {
	query := `
	INSERT INTO box_leases (username, box_id, started_at, expires_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(username) DO UPDATE SET
		box_id=excluded.box_id,
		started_at=excluded.started_at,
		expires_at=excluded.expires_at;
	`
	_, err := s.db.Exec(query, username, boxID, startedAt, expiresAt)
	return err
}

func (s *Store) DeleteBoxLease(username string) error {
	_, err := s.db.Exec("DELETE FROM box_leases WHERE username = ?", username)
	return err
}

func (s *Store) DeleteBoxLeaseByBoxID(boxID string) error {
	_, err := s.db.Exec("DELETE FROM box_leases WHERE box_id = ?", boxID)
	return err
}

func (s *Store) GetActiveBoxLeases() ([]BoxLeaseRecord, error) {
	rows, err := s.db.Query("SELECT username, box_id, started_at, expires_at FROM box_leases")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []BoxLeaseRecord
	for rows.Next() {
		var r BoxLeaseRecord
		if err := rows.Scan(&r.Username, &r.BoxID, &r.StartedAt, &r.ExpiresAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

// SessionRecord 表示持久化在 SQLite 中的用户 HTTP 会话凭据
type SessionRecord struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Store) SaveSession(token, username, role string, expiresAt time.Time) error {
	query := `
	INSERT INTO sessions (token, username, role, expires_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(token) DO UPDATE SET
		username=excluded.username,
		role=excluded.role,
		expires_at=excluded.expires_at;
	`
	_, err := s.db.Exec(query, token, username, role, expiresAt)
	return err
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (s *Store) DeleteSessionsByUsername(username string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE username = ?", username)
	return err
}

func (s *Store) GetActiveSessions() ([]SessionRecord, error) {
	rows, err := s.db.Query("SELECT token, username, role, expires_at FROM sessions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SessionRecord
	for rows.Next() {
		var r SessionRecord
		if err := rows.Scan(&r.Token, &r.Username, &r.Role, &r.ExpiresAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}
