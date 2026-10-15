package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists   = errors.New("username already exists")
	ErrUserNotFound = errors.New("user not found")
	ErrInvalidPass  = errors.New("invalid password")
	ErrUserDisabled = errors.New("user account is disabled")
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
	`
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	// 保证默认管理员账号存在
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin' OR username = 'laji'").Scan(&count)
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("qwer1234!"), bcrypt.DefaultCost)
		_, _ = s.db.Exec("INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, 'admin', 1)", "laji", string(hash))
		_, _ = s.db.Exec("INSERT INTO users (username, password_hash, role, active) VALUES (?, ?, 'admin', 1)", "admin", string(hash))
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
	if username == "" || password == "" {
		return nil, errors.New("username and password cannot be empty")
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
