package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"linuxtorouter/internal/database"
	"linuxtorouter/internal/models"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrUserExists       = errors.New("user already exists")
)

type UserService struct {
	db *database.DB
}

func NewUserService(db *database.DB) *UserService {
	return &UserService{db: db}
}

func (s *UserService) Create(username, password string, isAdmin bool) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	result, err := s.db.Exec(
		"INSERT INTO users (username, password_hash, is_admin) VALUES (?, ?, ?)",
		username, string(hash), isAdmin,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, _ := result.LastInsertId()
	return &models.User{
		ID:        id,
		Username:  username,
		IsAdmin:   isAdmin,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (s *UserService) Authenticate(username, password string) (*models.User, error) {
	user, err := s.GetByUsername(username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	return user, nil
}

func (s *UserService) GetByID(id int64) (*models.User, error) {
	var user models.User
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, is_admin, created_at, updated_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (s *UserService) GetByUsername(username string) (*models.User, error) {
	var user models.User
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, is_admin, created_at, updated_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (s *UserService) List() ([]models.User, error) {
	rows, err := s.db.Query(
		"SELECT id, username, password_hash, is_admin, created_at, updated_at FROM users ORDER BY username",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, nil
}

func (s *UserService) Update(id int64, password *string, isAdmin *bool) error {
	if password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		if _, err := s.db.Exec(
			"UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			string(hash), id,
		); err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}
	}

	if isAdmin != nil {
		if _, err := s.db.Exec(
			"UPDATE users SET is_admin = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			*isAdmin, id,
		); err != nil {
			return fmt.Errorf("failed to update admin status: %w", err)
		}
	}

	return nil
}

func (s *UserService) Delete(id int64) error {
	result, err := s.db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *UserService) Count() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (s *UserService) EnsureDefaultAdmin(username, password string) error {
	count, err := s.Count()
	if err != nil {
		return err
	}
	if count == 0 {
		_, err = s.Create(username, password, true)
		return err
	}
	return nil
}

func (s *UserService) LogAction(userID *int64, action, details, ipAddress string) error {
	_, err := s.db.Exec(
		"INSERT INTO audit_logs (user_id, action, details, ip_address) VALUES (?, ?, ?, ?)",
		userID, action, details, ipAddress,
	)
	return err
}

func (s *UserService) GetAuditLogs(limit int) ([]models.AuditLog, error) {
	rows, err := s.db.Query(`
		SELECT a.id, a.user_id, COALESCE(u.username, 'system'), a.action, a.details, a.ip_address, a.created_at
		FROM audit_logs a
		LEFT JOIN users u ON a.user_id = u.id
		ORDER BY a.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		if err := rows.Scan(&log.ID, &log.UserID, &log.Username, &log.Action, &log.Details, &log.IPAddress, &log.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && (
		contains(err.Error(), "UNIQUE constraint failed") ||
		contains(err.Error(), "duplicate key"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
