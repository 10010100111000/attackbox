package control

import (
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattn/go-sqlite3"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("username or email already exists")
)

// User represents a registered system user.
// The ID is strictly the database autoincrement unique primary key (int64).
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
	Status       int       `json:"status"` // 1: Active, 0: Suspended
	TokenVersion int       `json:"token_version"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuditLog records security and lifecycle events.
type AuditLog struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id"`
	Action    string    `json:"action"`
	IP        string    `json:"ip"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

// DatabaseStore encapsulates all SQLite database operations for users and logs.
type DatabaseStore struct {
	db *sql.DB
}

// NewDatabaseStore initializes the SQLite database with WAL mode and runs schema migrations.
func NewDatabaseStore(dbPath string) (*DatabaseStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}

	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_busy_timeout=5000", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

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

// Close closes the database handle.
func (s *DatabaseStore) Close() error {
	return s.db.Close()
}

func (s *DatabaseStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT UNIQUE NOT NULL COLLATE NOCASE,
		password_hash TEXT NOT NULL,
		email         TEXT UNIQUE NOT NULL COLLATE NOCASE,
		role          TEXT NOT NULL DEFAULT 'user',
		status        INTEGER NOT NULL DEFAULT 1,
		token_version INTEGER NOT NULL DEFAULT 1,
		created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_nocase ON users(username COLLATE NOCASE) WHERE username != '' AND username IS NOT NULL;
	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_nocase ON users(email COLLATE NOCASE) WHERE email != '' AND email IS NOT NULL;

	CREATE TABLE IF NOT EXISTS audit_logs (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id    INTEGER,
		action     TEXT NOT NULL,
		ip         TEXT,
		details    TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Backward compatibility migrations
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN status INTEGER NOT NULL DEFAULT 1;`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 1;`)

	return nil
}

// ============================================================================
// 1. USER CREATION (增)
// ============================================================================

// CreateUser registers a new user with an autoincrement unique primary key ID.
// If username is empty, it automatically derives USER_<id> based on the generated ID.
func (s *DatabaseStore) CreateUser(username, passwordHash, email, role string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, errors.New("invalid email address")
	}

	if role == "" {
		role = "user"
	}

	// 1. Proactively check email uniqueness
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM users WHERE email = ? COLLATE NOCASE`, email).Scan(&count); err == nil && count > 0 {
		return nil, ErrUserAlreadyExists
	}

	// 2. Resolve and validate optional custom username
	username = strings.TrimSpace(username)
	if username != "" {
		if len(username) < 2 || len(username) > 24 {
			return nil, errors.New("username length must be between 2 and 24 characters")
		}
		if err := s.db.QueryRow(`SELECT COUNT(1) FROM users WHERE username = ? COLLATE NOCASE`, username).Scan(&count); err == nil && count > 0 {
			return nil, ErrUserAlreadyExists
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var id int64
	if username != "" {
		// User specified a custom username upfront: single direct insert
		res, err := tx.Exec(`
			INSERT INTO users (username, password_hash, email, role, status, token_version)
			VALUES (?, ?, ?, ?, 1, 1)
		`, username, passwordHash, email, role)
		if err != nil {
			return nil, classifyInsertError(err)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return nil, err
		}
	} else {
		// User left username empty: insert with temporary placeholder then assign canonical USER_<id>
		tmpPlaceholder := fmt.Sprintf("__TMP_%d_%d__", time.Now().UnixNano(), rand.Int63())
		res, err := tx.Exec(`
			INSERT INTO users (username, password_hash, email, role, status, token_version)
			VALUES (?, ?, ?, ?, 1, 1)
		`, tmpPlaceholder, passwordHash, email, role)
		if err != nil {
			return nil, classifyInsertError(err)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return nil, err
		}
		defaultUsername := fmt.Sprintf("USER_%d", id)
		if _, err := tx.Exec(`UPDATE users SET username = ? WHERE id = ?`, defaultUsername, id); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return s.GetUserByID(id)
}

// EnsureAdminUser verifies or creates the initial system administrator.
func (s *DatabaseStore) EnsureAdminUser(username, passwordHash, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	username = strings.TrimSpace(username)

	// Check if any admin already exists
	var existingID int64
	var existingEmail string
	err := s.db.QueryRow(`SELECT id, email FROM users WHERE role = 'admin' LIMIT 1`).Scan(&existingID, &existingEmail)
	if err == nil {
		// Admin exists: synchronize email and passwordHash to align with configuration
		_, err = s.db.Exec(`UPDATE users SET email = ?, password_hash = ? WHERE id = ?`, email, passwordHash, existingID)
		return err
	}

	// No admin exists, create one directly
	_, err = s.CreateUser(username, passwordHash, email, "admin")
	return err
}

// ============================================================================
// 2. USER QUERY / READ (查)
// ============================================================================

// GetUserByID retrieves a user by their unique primary key ID.
func (s *DatabaseStore) GetUserByID(id int64) (*User, error) {
	query := `SELECT id, username, password_hash, email, role, status, token_version, created_at FROM users WHERE id = ?`
	row := s.db.QueryRow(query, id)
	return scanUser(row)
}

// GetUserByEmail retrieves a user by their registered email address (case-insensitive).
func (s *DatabaseStore) GetUserByEmail(email string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	query := `SELECT id, username, password_hash, email, role, status, token_version, created_at FROM users WHERE email = ? COLLATE NOCASE`
	row := s.db.QueryRow(query, email)
	return scanUser(row)
}

// GetUserByUsername retrieves a user by their username (case-insensitive).
func (s *DatabaseStore) GetUserByUsername(username string) (*User, error) {
	username = strings.TrimSpace(username)
	query := `SELECT id, username, password_hash, email, role, status, token_version, created_at FROM users WHERE username = ? COLLATE NOCASE`
	row := s.db.QueryRow(query, username)
	return scanUser(row)
}

// ListUsers returns paginated user records.
func (s *DatabaseStore) ListUsers(limit, offset int) ([]*User, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, username, password_hash, email, role, status, token_version, created_at
		FROM users
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var u User
		var createdAt string
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Email, &u.Role, &u.Status, &u.TokenVersion, &createdAt); err != nil {
			return nil, err
		}
		u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		users = append(users, &u)
	}
	return users, rows.Err()
}

// CountUsers returns the total count of registered users.
func (s *DatabaseStore) CountUsers() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM users`).Scan(&count)
	return count, err
}

// ============================================================================
// 3. USER UPDATE (改)
// ============================================================================

// UpdateUsername updates a user's display username after verifying uniqueness across the system.
func (s *DatabaseStore) UpdateUsername(userID int64, newUsername string) error {
	newUsername = strings.TrimSpace(newUsername)
	if len(newUsername) < 2 || len(newUsername) > 24 {
		return errors.New("username length must be between 2 and 24 characters")
	}

	var existingID int64
	err := s.db.QueryRow(`SELECT id FROM users WHERE username = ? COLLATE NOCASE`, newUsername).Scan(&existingID)
	if err == nil {
		if existingID != userID {
			return ErrUserAlreadyExists
		}
		return nil // Already assigned to this user, no-op
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = s.db.Exec(`UPDATE users SET username = ? WHERE id = ?`, newUsername, userID)
	if err != nil {
		return classifyInsertError(err)
	}
	return nil
}

// UpdatePassword changes a user's password hash and invalidates existing JWT sessions.
func (s *DatabaseStore) UpdatePassword(userID int64, newPasswordHash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ?, token_version = token_version + 1 WHERE id = ?`, newPasswordHash, userID)
	return err
}

// UpdateStatus activates or suspends a user account.
func (s *DatabaseStore) UpdateStatus(userID int64, status int) error {
	_, err := s.db.Exec(`UPDATE users SET status = ?, token_version = token_version + 1 WHERE id = ?`, status, userID)
	return err
}

// UpdateRole changes a user's authorization role.
func (s *DatabaseStore) UpdateRole(userID int64, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, userID)
	return err
}

// IncrementTokenVersion revokes all existing JWTs for a user.
func (s *DatabaseStore) IncrementTokenVersion(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET token_version = token_version + 1 WHERE id = ?`, userID)
	return err
}

// ============================================================================
// 4. USER DELETION (删)
// ============================================================================

// DeleteUser permanently deletes a user record by their ID.
func (s *DatabaseStore) DeleteUser(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, userID)
	return err
}

// ============================================================================
// 5. AUDIT LOGS
// ============================================================================

// AddAuditLog records administrative and security audit events.
func (s *DatabaseStore) AddAuditLog(userID *int64, action, ip, details string) error {
	_, err := s.db.Exec(`INSERT INTO audit_logs (user_id, action, ip, details) VALUES (?, ?, ?, ?)`,
		userID, action, ip, details)
	return err
}

// ListAuditLogs retrieves paginated audit logs.
func (s *DatabaseStore) ListAuditLogs(limit, offset int) ([]*AuditLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT id, user_id, action, ip, details, created_at
		FROM audit_logs
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`
	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		var a AuditLog
		var createdAt string
		if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.IP, &a.Details, &createdAt); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		logs = append(logs, &a)
	}
	return logs, rows.Err()
}

// ============================================================================
// HELPERS
// ============================================================================

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

func classifyInsertError(err error) error {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code == sqlite3.ErrConstraint {
		return fmt.Errorf("%w: %v", ErrUserAlreadyExists, err)
	}
	return fmt.Errorf("database constraint: %w", err)
}
