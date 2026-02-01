package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/ritik/twitter-fan-out/internal/models"
)

// FollowRepository handles follow-related database operations
type FollowRepository struct {
	db *sqlx.DB
}

// NewFollowRepository creates a new FollowRepository
func NewFollowRepository(db *sqlx.DB) *FollowRepository {
	return &FollowRepository{db: db}
}

// Create creates a new follow relationship
func (r *FollowRepository) Create(ctx context.Context, followerID, followeeID int64) error {
	query := `INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	_, err := r.db.ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("failed to create follow: %w", err)
	}
	return nil
}

// Delete removes a follow relationship
func (r *FollowRepository) Delete(ctx context.Context, followerID, followeeID int64) error {
	query := `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`
	_, err := r.db.ExecContext(ctx, query, followerID, followeeID)
	if err != nil {
		return fmt.Errorf("failed to delete follow: %w", err)
	}
	return nil
}

// GetFollowers retrieves all followers of a user
func (r *FollowRepository) GetFollowers(ctx context.Context, userID int64) ([]int64, error) {
	query := `SELECT follower_id FROM follows WHERE followee_id = $1`
	var followers []int64
	err := r.db.SelectContext(ctx, &followers, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get followers: %w", err)
	}
	return followers, nil
}

// GetFollowing retrieves all users that a user follows
func (r *FollowRepository) GetFollowing(ctx context.Context, userID int64) ([]int64, error) {
	query := `SELECT followee_id FROM follows WHERE follower_id = $1`
	var following []int64
	err := r.db.SelectContext(ctx, &following, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get following: %w", err)
	}
	return following, nil
}

// GetFollowingUsers retrieves all users that a user follows with full user data
func (r *FollowRepository) GetFollowingUsers(ctx context.Context, userID int64) ([]*models.User, error) {
	query := `
		SELECT u.id, u.username, u.follower_count, u.following_count, u.created_at
		FROM users u
		JOIN follows f ON u.id = f.followee_id
		WHERE f.follower_id = $1
	`
	users := []*models.User{}
	err := r.db.SelectContext(ctx, &users, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get following users: %w", err)
	}
	return users, nil
}

// GetFollowingCelebrities retrieves celebrities that a user follows
func (r *FollowRepository) GetFollowingCelebrities(ctx context.Context, userID int64, threshold int) ([]*models.User, error) {
	query := `
		SELECT u.id, u.username, u.follower_count, u.following_count, u.created_at
		FROM users u
		JOIN follows f ON u.id = f.followee_id
		WHERE f.follower_id = $1 AND u.follower_count >= $2
	`
	users := []*models.User{}
	err := r.db.SelectContext(ctx, &users, query, userID, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to get following celebrities: %w", err)
	}
	return users, nil
}

// GetFollowingNonCelebrities retrieves non-celebrities that a user follows
func (r *FollowRepository) GetFollowingNonCelebrities(ctx context.Context, userID int64, threshold int) ([]int64, error) {
	query := `
		SELECT u.id
		FROM users u
		JOIN follows f ON u.id = f.followee_id
		WHERE f.follower_id = $1 AND u.follower_count < $2
	`
	var userIDs []int64
	err := r.db.SelectContext(ctx, &userIDs, query, userID, threshold)
	if err != nil {
		return nil, fmt.Errorf("failed to get following non-celebrities: %w", err)
	}
	return userIDs, nil
}

// IsFollowing checks if a user follows another user
func (r *FollowRepository) IsFollowing(ctx context.Context, followerID, followeeID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2)`
	var exists bool
	err := r.db.GetContext(ctx, &exists, query, followerID, followeeID)
	if err != nil {
		return false, fmt.Errorf("failed to check follow: %w", err)
	}
	return exists, nil
}

// Count returns the total number of follow relationships
func (r *FollowRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM follows")
	if err != nil {
		return 0, fmt.Errorf("failed to count follows: %w", err)
	}
	return count, nil
}

// BulkCreate creates multiple follow relationships efficiently
func (r *FollowRepository) BulkCreate(ctx context.Context, follows []struct {
	FollowerID int64
	FolloweeID int64
}) error {
	if len(follows) == 0 {
		return nil
	}

	// Build bulk insert query
	valueStrings := make([]string, 0, len(follows))
	valueArgs := make([]interface{}, 0, len(follows)*2)
	
	for i, f := range follows {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		valueArgs = append(valueArgs, f.FollowerID, f.FolloweeID)
	}

	query := fmt.Sprintf("INSERT INTO follows (follower_id, followee_id) VALUES %s ON CONFLICT DO NOTHING", strings.Join(valueStrings, ","))
	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("failed to bulk create follows: %w", err)
	}
	return nil
}

// Truncate removes all follows (for testing/reset)
func (r *FollowRepository) Truncate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "TRUNCATE follows CASCADE")
	if err != nil {
		return fmt.Errorf("failed to truncate follows: %w", err)
	}
	return nil
}
