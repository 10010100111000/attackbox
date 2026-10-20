package control

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("username or email already exists")
)

// User represents a registered participant or administrator.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	Status       int       `json:"status"`
	TokenVersion int       `json:"token_version"`
	CreatedAt    time.Time `json:"created_at"`
}

// DatabaseStore encapsulates SQLite operations.
type DatabaseStore struct {
	db *sql.DB
}

// NewDatabaseStore initializes the SQLite database with WAL mode and runs DDL migrations.
func NewDatabaseStore(dbPath string) (*DatabaseStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}

	// Connect with WAL mode and 5000ms busy timeout
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// WAL mode with _busy_timeout enables concurrent readers alongside writers
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(1 * time.Hour)

	s := &DatabaseStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	return s, nil
}

// Close closes the underlying database handle.
func (s *DatabaseStore) Close() error {
	return s.db.Close()
}

func (s *DatabaseStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		email         TEXT UNIQUE NOT NULL,
		role          TEXT NOT NULL DEFAULT 'user',
		status        INTEGER NOT NULL DEFAULT 1,
		token_version INTEGER NOT NULL DEFAULT 1,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    INTEGER,
		action     TEXT NOT NULL,
		ip         TEXT,
		details    TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := s.db.Exec(schema)
	return err
}

// CreateUser registers a new user. If role is empty, the first user in the database is automatically
// assigned 'admin' for system bootstrapping.
func (s *DatabaseStore) CreateUser(username, passwordHash, email string, role string) (*User, error) {
	if role == "" {
		var count int
		_ = s.db.QueryRow(`SELECT COUNT(1) FROM users`).Scan(&count)
		if count == 0 {
			role = "admin"
		} else {
			role = "user"
		}
	}

	query := `
	INSERT INTO users (username, password_hash, email, role)
	VALUES (?, ?, ?, ?)
	`
	res, err := s.db.Exec(query, username, passwordHash, email, role)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserAlreadyExists, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetUserByID(id)
}

// EnsureAdminUser bootstraps an initial administrator account if no admin currently exists.
func (s *DatabaseStore) EnsureAdminUser(username, passwordHash, email string) error {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(1) FROM users WHERE role = 'admin'`).Scan(&count)
	if count > 0 {
		return nil // Admin already present
	}

	_, err := s.db.Exec(`INSERT INTO users (username, password_hash, email, role) VALUES (?, ?, ?, 'admin')`, username, passwordHash, email)
	return err
}

// GetUserByUsername retrieves a user by their username.
func (s *DatabaseStore) GetUserByUsername(username string) (*User, error) {
	query := `SELECT id, username, password_hash, email, role, status, token_version, created_at FROM users WHERE username = ?`
	row := s.db.QueryRow(query, username)
	return scanUser(row)
}

// GetUserByID retrieves a user by their unique ID.
func (s *DatabaseStore) GetUserByID(id int64) (*User, error) {
	query := `SELECT id, username, password_hash, email, role, status, token_version, created_at FROM users WHERE id = ?`
	row := s.db.QueryRow(query, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var createdAt string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Role, &u.Status, &u.TokenVersion, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	return &u, nil
}

// IncrementTokenVersion revokes all existing JWTs for a user.
func (s *DatabaseStore) IncrementTokenVersion(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET token_version = token_version + 1 WHERE id = ?`, userID)
	return err
}

// AddAuditLog records critical administrative and user security events.
func (s *DatabaseStore) AddAuditLog(userID *int64, action, ip, details string) error {
	_, err := s.db.Exec(`INSERT INTO audit_logs (user_id, action, ip, details) VALUES (?, ?, ?, ?)`,
		userID, action, ip, details)
	return err
}

