package auth

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	Email        string    `json:"email"`
	AffCode      string    `json:"aff_code"`
	InvitedBy    string    `json:"invited_by"`
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

	-- 移除旧架构残留的持久化租约表，运行时状态已彻底收敛至纯内存
	DROP TABLE IF EXISTS box_leases;

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		username TEXT NOT NULL,
		role TEXT NOT NULL,
		expires_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		event_type TEXT NOT NULL,
		actor TEXT NOT NULL,
		target TEXT NOT NULL,
		ip TEXT NOT NULL,
		details TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_logs_event ON audit_logs(event_type);

	CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS oauth_configs (
		provider TEXT PRIMARY KEY,
		client_id TEXT NOT NULL,
		encrypted_secret TEXT NOT NULL,
		nonce TEXT NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 0,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS user_oauth_bindings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		provider TEXT NOT NULL,
		provider_uid TEXT NOT NULL,
		provider_email TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(provider, provider_uid)
	);
	CREATE INDEX IF NOT EXISTS idx_oauth_provider_uid ON user_oauth_bindings(provider, provider_uid);
	`
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}

	// 保证默认管理员账号存在 (若不存在则随机生成一次性口令并输出到日志)
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count); err != nil {
		return fmt.Errorf("check admin exists: %w", err)
	}
	if count == 0 {
		rawPass, err := generateRandomPassword(16)
		if err != nil {
			return fmt.Errorf("generate random admin password: %w", err)
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(rawPass), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash default admin password: %w", err)
		}
		if _, err := s.db.Exec("INSERT INTO users (username, password_hash, role) VALUES ('admin', ?, 'admin')", string(hash)); err != nil {
			return fmt.Errorf("insert default admin: %w", err)
		}
		log.Printf("\n==========================================================\n[SECURITY] Initial admin account initialized!\nUsername: admin\nOne-Time Password: %s\nPlease change this password immediately after logging in.\n==========================================================\n", rawPass)
	}

	// 自动平滑迁移 users 表：email, aff_code, invited_by
	if err := s.migrateUsersTable(); err != nil {
		log.Printf("warning: migrate users table: %v", err)
	}

	return nil
}

// generateRandomPassword 生成无模数偏差（Unbiased）的高熵随机口令
func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%^&*"
	maxBig := big.NewInt(int64(len(charset)))
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, maxBig)
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// generateAFFCode 生成无歧义大写字母与数字的 8 位专属邀请码
func generateAFFCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 剔除 0, O, 1, I
	maxBig := big.NewInt(int64(len(charset)))
	b := make([]byte, 8)
	for i := range b {
		n, _ := rand.Int(rand.Reader, maxBig)
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// migrateUsersTable 迁移扩展 users 表字段并为老用户补充专属 AFF 邀请码
func (s *Store) migrateUsersTable() error {
	_, _ = s.db.Exec("ALTER TABLE users ADD COLUMN email TEXT DEFAULT ''")
	_, _ = s.db.Exec("ALTER TABLE users ADD COLUMN aff_code TEXT DEFAULT ''")
	_, _ = s.db.Exec("ALTER TABLE users ADD COLUMN invited_by TEXT DEFAULT ''")
	_, _ = s.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_aff_code ON users(aff_code) WHERE aff_code != ''")
	_, _ = s.db.Exec("CREATE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email != ''")

	// 补齐存量用户缺失的 aff_code
	rows, err := s.db.Query("SELECT id FROM users WHERE aff_code = '' OR aff_code IS NULL")
	if err == nil {
		defer rows.Close()
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		for _, id := range ids {
			code := generateAFFCode()
			_, _ = s.db.Exec("UPDATE users SET aff_code = ? WHERE id = ?", code, id)
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
	query := "SELECT id, username, password_hash, role, email, aff_code, invited_by, active, created_at FROM users WHERE username = ?"
	err := s.db.QueryRow(query, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.Email, &u.AffCode, &u.InvitedBy, &activeInt, &u.CreatedAt)
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
	return s.CreateUserWithEmailAndAFF(username, password, role, "", "")
}

func (s *Store) CreateUserWithEmailAndAFF(username, password, role, email, invitedBy string) (*User, error) {
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

	affCode := generateAFFCode()
	query := "INSERT INTO users (username, password_hash, role, email, aff_code, invited_by, active) VALUES (?, ?, ?, ?, ?, ?, 1)"
	res, err := s.db.Exec(query, username, string(hash), role, strings.TrimSpace(email), affCode, strings.TrimSpace(invitedBy))
	if err != nil {
		return nil, ErrUserExists
	}

	id, _ := res.LastInsertId()
	return &User{
		ID:        id,
		Username:  username,
		Role:      role,
		Email:     email,
		AffCode:   affCode,
		InvitedBy: invitedBy,
		Active:    true,
		CreatedAt: time.Now(),
	}, nil
}

func (s *Store) ListUsers() ([]User, error) {
	query := "SELECT id, username, role, email, aff_code, invited_by, active, created_at FROM users ORDER BY id ASC"
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []User
	for rows.Next() {
		var u User
		var activeInt int
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Email, &u.AffCode, &u.InvitedBy, &activeInt, &u.CreatedAt); err != nil {
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
	query := "SELECT id, username, role, email, aff_code, invited_by, active, created_at FROM users WHERE username = ?"
	err := s.db.QueryRow(query, username).Scan(&u.ID, &u.Username, &u.Role, &u.Email, &u.AffCode, &u.InvitedBy, &activeInt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	u.Active = (activeInt == 1)
	return &u, nil
}

func (s *Store) GetUserByEmail(email string) (*User, error) {
	if email == "" {
		return nil, ErrUserNotFound
	}
	var u User
	var activeInt int
	query := "SELECT id, username, role, email, aff_code, invited_by, active, created_at FROM users WHERE email = ?"
	err := s.db.QueryRow(query, email).Scan(&u.ID, &u.Username, &u.Role, &u.Email, &u.AffCode, &u.InvitedBy, &activeInt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	u.Active = (activeInt == 1)
	return &u, nil
}

func (s *Store) GetUserByAffCode(affCode string) (*User, error) {
	if affCode == "" {
		return nil, ErrUserNotFound
	}
	var u User
	var activeInt int
	query := "SELECT id, username, role, email, aff_code, invited_by, active, created_at FROM users WHERE aff_code = ?"
	err := s.db.QueryRow(query, affCode).Scan(&u.ID, &u.Username, &u.Role, &u.Email, &u.AffCode, &u.InvitedBy, &activeInt, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	u.Active = (activeInt == 1)
	return &u, nil
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

// AuditLog 表示关键安全审计记录
type AuditLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	Actor     string    `json:"actor"`
	Target    string    `json:"target"`
	IP        string    `json:"ip"`
	Details   string    `json:"details"`
}

// RecordAudit 记录一条不可篡改的安全审计日志
func (s *Store) RecordAudit(eventType, actor, target, ip, details string) error {
	query := "INSERT INTO audit_logs (event_type, actor, target, ip, details) VALUES (?, ?, ?, ?, ?)"
	_, err := s.db.Exec(query, eventType, actor, target, ip, details)
	return err
}

// ListAuditLogs 获取审计日志，支持根据 eventType 和 search 模糊搜索，以及分页
func (s *Store) ListAuditLogs(limit, offset int, eventType, search string) ([]AuditLog, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	whereClauses := []string{"1=1"}
	var args []any

	if eventType != "" {
		whereClauses = append(whereClauses, "event_type = ?")
		args = append(args, eventType)
	}
	if search != "" {
		whereClauses = append(whereClauses, "(actor LIKE ? OR target LIKE ? OR ip LIKE ? OR details LIKE ?)")
		pat := "%" + search + "%"
		args = append(args, pat, pat, pat, pat)
	}

	whereStr := " WHERE " + strings.Join(whereClauses, " AND ")

	// 统计总数
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs" + whereStr
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 分页查询
	query := "SELECT id, timestamp, event_type, actor, target, ip, details FROM audit_logs" +
		whereStr + " ORDER BY id DESC LIMIT ? OFFSET ?"
	queryArgs := append(args, limit, offset)

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		if err := rows.Scan(&l.ID, &l.Timestamp, &l.EventType, &l.Actor, &l.Target, &l.IP, &l.Details); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}

// GetSetting 读取持久化配置项，若未设置则返回默认值
func (s *Store) GetSetting(key, defVal string) string {
	var val string
	if err := s.db.QueryRow("SELECT value FROM system_settings WHERE key = ?", key).Scan(&val); err != nil {
		return defVal
	}
	return val
}

// SetSetting 保存持久化配置项
func (s *Store) SetSetting(key, val string) error {
	query := `
	INSERT INTO system_settings (key, value)
	VALUES (?, ?)
	ON CONFLICT(key) DO UPDATE SET value = excluded.value;
	`
	_, err := s.db.Exec(query, key, val)
	return err
}

// GetAllSettings 批量读取所有已保存的全局配置项
func (s *Store) GetAllSettings() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM system_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			res[k] = v
		}
	}
	return res, nil
}

// PruneAuditLogs 定期保留最新记录，修剪陈旧审计记录以防止磁盘无节制增长
func (s *Store) PruneAuditLogs(keepMaxRows int) error {
	if keepMaxRows <= 0 {
		keepMaxRows = 50000
	}
	query := `
	DELETE FROM audit_logs WHERE id IN (
		SELECT id FROM audit_logs ORDER BY id DESC LIMIT -1 OFFSET ?
	);
	`
	_, err := s.db.Exec(query, keepMaxRows)
	return err
}

// OAuthConfig 第三方平台 OAuth 凭据配置模型 (Secret 已被 AES-256-GCM 加密落库)
type OAuthConfig struct {
	Provider        string    `json:"provider"`
	ClientID        string    `json:"client_id"`
	EncryptedSecret string    `json:"-"`
	Nonce           string    `json:"-"`
	Enabled         bool      `json:"enabled"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UserOAuthBinding 用户与第三方平台 UID 的绑定映射
type UserOAuthBinding struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Provider      string    `json:"provider"`
	ProviderUID   string    `json:"provider_uid"`
	ProviderEmail string    `json:"provider_email"`
	CreatedAt     time.Time `json:"created_at"`
}

// SaveOAuthConfig 写入或更新 OAuth 第三方配置 (密文落库)
func (s *Store) SaveOAuthConfig(provider, clientID, encryptedSecret, nonce string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	query := `
	INSERT INTO oauth_configs (provider, client_id, encrypted_secret, nonce, enabled, updated_at)
	VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT(provider) DO UPDATE SET
		client_id = excluded.client_id,
		encrypted_secret = CASE WHEN excluded.encrypted_secret != '' THEN excluded.encrypted_secret ELSE oauth_configs.encrypted_secret END,
		nonce = CASE WHEN excluded.nonce != '' THEN excluded.nonce ELSE oauth_configs.nonce END,
		enabled = excluded.enabled,
		updated_at = CURRENT_TIMESTAMP;
	`
	_, err := s.db.Exec(query, provider, clientID, encryptedSecret, nonce, enabledInt)
	return err
}

// GetOAuthConfig 读取指定 provider 的 OAuth 配置
func (s *Store) GetOAuthConfig(provider string) (*OAuthConfig, error) {
	var cfg OAuthConfig
	var enabledInt int
	query := `SELECT provider, client_id, encrypted_secret, nonce, enabled, updated_at FROM oauth_configs WHERE provider = ?`
	err := s.db.QueryRow(query, provider).Scan(&cfg.Provider, &cfg.ClientID, &cfg.EncryptedSecret, &cfg.Nonce, &enabledInt, &cfg.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	cfg.Enabled = (enabledInt == 1)
	return &cfg, nil
}

// ListOAuthConfigs 列出所有第三方 OAuth 配置
func (s *Store) ListOAuthConfigs() ([]*OAuthConfig, error) {
	rows, err := s.db.Query("SELECT provider, client_id, encrypted_secret, nonce, enabled, updated_at FROM oauth_configs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*OAuthConfig
	for rows.Next() {
		var cfg OAuthConfig
		var enabledInt int
		if err := rows.Scan(&cfg.Provider, &cfg.ClientID, &cfg.EncryptedSecret, &cfg.Nonce, &enabledInt, &cfg.UpdatedAt); err == nil {
			cfg.Enabled = (enabledInt == 1)
			list = append(list, &cfg)
		}
	}
	return list, nil
}

// GetUserByOAuth 根据第三方 provider 与 provider_uid 查询绑定的本地用户
func (s *Store) GetUserByOAuth(provider, providerUID string) (*User, error) {
	query := `
	SELECT u.id, u.username, u.password_hash, u.role, u.active, u.created_at
	FROM users u
	INNER JOIN user_oauth_bindings b ON u.id = b.user_id
	WHERE b.provider = ? AND b.provider_uid = ?
	`
	var u User
	var activeInt int
	err := s.db.QueryRow(query, provider, providerUID).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &activeInt, &u.CreatedAt)
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
	return &u, nil
}

// CreateUserWithOAuthBinding 事务执行：创建新用户并与第三方身份关联
func (s *Store) CreateUserWithOAuthBinding(username, password, provider, providerUID, providerEmail string) (*User, error) {
	if !usernameRegex.MatchString(username) {
		return nil, ErrInvalidUsername
	}

	// 若未指定密码，生成高熵强口令占位入库，确保密码哈希字段非空
	rawPass := password
	if rawPass == "" {
		p, err := generateRandomPassword(24)
		if err != nil {
			return nil, fmt.Errorf("generate random placeholder password: %w", err)
		}
		rawPass = p
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(rawPass), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	affCode := generateAFFCode()
	// 1. 插入 users 表
	res, err := tx.Exec("INSERT INTO users (username, password_hash, role, email, aff_code, active) VALUES (?, ?, 'user', ?, ?, 1)",
		username, string(hash), strings.TrimSpace(providerEmail), affCode)
	if err != nil {
		return nil, ErrUserExists
	}

	userID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	// 2. 插入 user_oauth_bindings 表
	_, err = tx.Exec("INSERT INTO user_oauth_bindings (user_id, provider, provider_uid, provider_email) VALUES (?, ?, ?, ?)",
		userID, provider, providerUID, providerEmail)
	if err != nil {
		return nil, fmt.Errorf("insert oauth binding: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &User{
		ID:        userID,
		Username:  username,
		Role:      "user",
		Email:     providerEmail,
		AffCode:   affCode,
		Active:    true,
		CreatedAt: time.Now(),
	}, nil
}


