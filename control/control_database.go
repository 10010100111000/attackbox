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
	ErrUserNotFound       = errors.New("user not found")
	ErrUserAlreadyExists  = errors.New("username or email already exists")
	ErrCursorAlreadyUsed  = errors.New("point transaction cursor already processed")
	ErrInsufficientPoints = errors.New("insufficient points balance")
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
	InviteCode   string    `json:"invite_code"`
	InvitedBy    *int64    `json:"invited_by,omitempty"`
	Points       int       `json:"points"`
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

	// Single writer connection pool for SQLite concurrency safety
	db.SetMaxOpenConns(1)

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
		invite_code   TEXT UNIQUE NOT NULL,
		invited_by    INTEGER,
		points        INTEGER NOT NULL DEFAULT 100,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS point_transactions (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		cursor_id   TEXT UNIQUE NOT NULL,
		user_id     INTEGER NOT NULL,
		amount      INTEGER NOT NULL,
		action      TEXT NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_transactions_user ON point_transactions(user_id);

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

// CreateUser registers a new user with initial points.
func (s *DatabaseStore) CreateUser(username, passwordHash, email, inviteCode string, invitedBy *int64) (*User, error) {
	query := `
	INSERT INTO users (username, password_hash, email, invite_code, invited_by, points)
	VALUES (?, ?, ?, ?, ?, 100)
	`
	res, err := s.db.Exec(query, username, passwordHash, email, inviteCode, invitedBy)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserAlreadyExists, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetUserByID(id)
}

// GetUserByUsername retrieves a user by their username.
func (s *DatabaseStore) GetUserByUsername(username string) (*User, error) {
	query := `SELECT id, username, password_hash, email, role, status, token_version, invite_code, invited_by, points, created_at FROM users WHERE username = ?`
	row := s.db.QueryRow(query, username)
	return scanUser(row)
}

// GetUserByID retrieves a user by their unique ID.
func (s *DatabaseStore) GetUserByID(id int64) (*User, error) {
	query := `SELECT id, username, password_hash, email, role, status, token_version, invite_code, invited_by, points, created_at FROM users WHERE id = ?`
	row := s.db.QueryRow(query, id)
	return scanUser(row)
}

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var createdAt string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Role, &u.Status, &u.TokenVersion, &u.InviteCode, &u.InvitedBy, &u.Points, &createdAt)
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

// DeductPointsWithCursor atomically deducts points using an idempotency cursor.
func (s *DatabaseStore) DeductPointsWithCursor(cursorID string, userID int64, amount int, action string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// 1. Check idempotency cursor
	var exists int
	err = tx.QueryRow(`SELECT 1 FROM point_transactions WHERE cursor_id = ?`, cursorID).Scan(&exists)
	if err == nil {
		return 0, ErrCursorAlreadyUsed
	}

	// 2. Check current balance
	var balance int
	err = tx.QueryRow(`SELECT points FROM users WHERE id = ?`, userID).Scan(&balance)
	if err != nil {
		return 0, ErrUserNotFound
	}
	if balance < amount {
		return balance, ErrInsufficientPoints
	}

	// 3. Deduct points
	newBalance := balance - amount
	_, err = tx.Exec(`UPDATE users SET points = ? WHERE id = ?`, newBalance, userID)
	if err != nil {
		return 0, err
	}

	// 4. Record transaction cursor
	_, err = tx.Exec(`INSERT INTO point_transactions (cursor_id, user_id, amount, action) VALUES (?, ?, ?, ?)`,
		cursorID, userID, amount, action)
	if err != nil {
		return 0, err
	}

	return newBalance, tx.Commit()
}

// AddAuditLog records critical administrative and user security events.
func (s *DatabaseStore) AddAuditLog(userID *int64, action, ip, details string) error {
	_, err := s.db.Exec(`INSERT INTO audit_logs (user_id, action, ip, details) VALUES (?, ?, ?, ?)`,
		userID, action, ip, details)
	return err
}
