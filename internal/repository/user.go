package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/ritik/twitter-fan-out/internal/models"
)

// UserRepository handles user-related database operations
type UserRepository struct {
	db *sqlx.DB
}

// NewUserRepository creates a new UserRepository
func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, username string) (*models.User, error) {
	query := `
		INSERT INTO users (username)
		VALUES ($1)
		RETURNING id, username, follower_count, following_count, created_at
	`
	user := &models.User{}
	err := r.db.QueryRowxContext(ctx, query, username).StructScan(user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	query := `SELECT id, username, follower_count, following_count, created_at FROM users WHERE id = $1`
	user := &models.User{}
	err := r.db.GetContext(ctx, user, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `SELECT id, username, follower_count, following_count, created_at FROM users WHERE username = $1`
	user := &models.User{}
	err := r.db.GetContext(ctx, user, query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetAll retrieves all users with pagination
func (r *UserRepository) GetAll(ctx context.Context, limit, offset int) ([]*models.User, error) {
	query := `SELECT id, username, follower_count, following_count, created_at FROM users ORDER BY id LIMIT $1 OFFSET $2`
	users := []*models.User{}
	err := r.db.SelectContext(ctx, &users, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	return users, nil
}

// GetCelebrities retrieves users with follower count above threshold
func (r *UserRepository) GetCelebrities(ctx context.Context, threshold int) ([]*models.User, error) {
	query := `
		SELECT id, username, follower_count, following_count, created_at 
		FROM users 
		WHERE follower_count >= $1 
		ORDER BY follower_count DESC
	`
	users := []*models.User{}
	err := r.db.SelectContext(ctx, &users, query, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to get celebrities: %w", err)
	}
	return users, nil
}

// GetRandomUsers retrieves random users for benchmarking
func (r *UserRepository) GetRandomUsers(ctx context.Context, count int) ([]*models.User, error) {
	query := `
		SELECT id, username, follower_count, following_count, created_at 
		FROM users 
		ORDER BY RANDOM() 
		LIMIT $1
	`
	users := []*models.User{}
	err := r.db.SelectContext(ctx, &users, query, count)
	if err != nil {
		return nil, fmt.Errorf("failed to get random users: %w", err)
	}
	return users, nil
}

// Count returns the total number of users
func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM users")
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// CountCelebrities returns the number of celebrities
func (r *UserRepository) CountCelebrities(ctx context.Context, threshold int) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM users WHERE follower_count >= $1", threshold)
	if err != nil {
		return 0, fmt.Errorf("failed to count celebrities: %w", err)
	}
	return count, nil
}

// BulkCreate creates multiple users efficiently
func (r *UserRepository) BulkCreate(ctx context.Context, usernames []string) error {
	if len(usernames) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PreparexContext(ctx, "INSERT INTO users (username) VALUES ($1) ON CONFLICT (username) DO NOTHING")
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, username := range usernames {
		_, err := stmt.ExecContext(ctx, username)
		if err != nil {
			return fmt.Errorf("failed to insert user %s: %w", username, err)
		}
	}

	return tx.Commit()
}

// Delete deletes a user
func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// Truncate removes all users (for testing/reset)
func (r *UserRepository) Truncate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "TRUNCATE users CASCADE")
	if err != nil {
		return fmt.Errorf("failed to truncate users: %w", err)
	}
	return nil
}
