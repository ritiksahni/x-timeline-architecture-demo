package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/ritik/twitter-fan-out/internal/models"
)

// TweetRepository handles tweet-related database operations
type TweetRepository struct {
	db *sqlx.DB
}

// NewTweetRepository creates a new TweetRepository
func NewTweetRepository(db *sqlx.DB) *TweetRepository {
	return &TweetRepository{db: db}
}

// Create creates a new tweet
func (r *TweetRepository) Create(ctx context.Context, userID int64, content string) (*models.Tweet, error) {
	query := `
		INSERT INTO tweets (user_id, content)
		VALUES ($1, $2)
		RETURNING id, user_id, content, created_at
	`
	tweet := &models.Tweet{}
	err := r.db.QueryRowxContext(ctx, query, userID, content).StructScan(tweet)
	if err != nil {
		return nil, fmt.Errorf("failed to create tweet: %w", err)
	}
	return tweet, nil
}

// GetByID retrieves a tweet by ID
func (r *TweetRepository) GetByID(ctx context.Context, id int64) (*models.Tweet, error) {
	query := `
		SELECT t.id, t.user_id, t.content, t.created_at, u.username
		FROM tweets t
		JOIN users u ON t.user_id = u.id
		WHERE t.id = $1
	`
	tweet := &models.Tweet{}
	err := r.db.GetContext(ctx, tweet, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get tweet: %w", err)
	}
	return tweet, nil
}

// GetByIDs retrieves multiple tweets by IDs
func (r *TweetRepository) GetByIDs(ctx context.Context, ids []int64) ([]*models.Tweet, error) {
	if len(ids) == 0 {
		return []*models.Tweet{}, nil
	}

	query := `
		SELECT t.id, t.user_id, t.content, t.created_at, u.username
		FROM tweets t
		JOIN users u ON t.user_id = u.id
		WHERE t.id = ANY($1)
		ORDER BY t.created_at DESC
	`
	tweets := []*models.Tweet{}
	err := r.db.SelectContext(ctx, &tweets, query, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get tweets: %w", err)
	}
	return tweets, nil
}

// GetByUserID retrieves tweets by user ID
func (r *TweetRepository) GetByUserID(ctx context.Context, userID int64, limit int) ([]*models.Tweet, error) {
	query := `
		SELECT t.id, t.user_id, t.content, t.created_at, u.username
		FROM tweets t
		JOIN users u ON t.user_id = u.id
		WHERE t.user_id = $1
		ORDER BY t.created_at DESC
		LIMIT $2
	`
	tweets := []*models.Tweet{}
	err := r.db.SelectContext(ctx, &tweets, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get tweets: %w", err)
	}
	return tweets, nil
}

// GetByUserIDs retrieves tweets from multiple users (for fan-out-read)
func (r *TweetRepository) GetByUserIDs(ctx context.Context, userIDs []int64, limit int) ([]*models.Tweet, error) {
	if len(userIDs) == 0 {
		return []*models.Tweet{}, nil
	}

	query := `
		SELECT t.id, t.user_id, t.content, t.created_at, u.username
		FROM tweets t
		JOIN users u ON t.user_id = u.id
		WHERE t.user_id = ANY($1)
		ORDER BY t.created_at DESC
		LIMIT $2
	`
	tweets := []*models.Tweet{}
	err := r.db.SelectContext(ctx, &tweets, query, userIDs, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get tweets: %w", err)
	}
	return tweets, nil
}

// GetRecentByUserIDs retrieves recent tweets from multiple users with per-user limit
// This is more efficient for fan-out-read when we need recent tweets from many users
func (r *TweetRepository) GetRecentByUserIDs(ctx context.Context, userIDs []int64, perUserLimit, totalLimit int) ([]*models.Tweet, error) {
	if len(userIDs) == 0 {
		return []*models.Tweet{}, nil
	}

	// Use a lateral join to get top N tweets per user, then sort overall
	query := `
		SELECT t.id, t.user_id, t.content, t.created_at, u.username
		FROM unnest($1::bigint[]) AS uid(id)
		CROSS JOIN LATERAL (
			SELECT id, user_id, content, created_at
			FROM tweets
			WHERE user_id = uid.id
			ORDER BY created_at DESC
			LIMIT $2
		) t
		JOIN users u ON t.user_id = u.id
		ORDER BY t.created_at DESC
		LIMIT $3
	`
	tweets := []*models.Tweet{}
	err := r.db.SelectContext(ctx, &tweets, query, userIDs, perUserLimit, totalLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent tweets: %w", err)
	}
	return tweets, nil
}

// Count returns the total number of tweets
func (r *TweetRepository) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM tweets")
	if err != nil {
		return 0, fmt.Errorf("failed to count tweets: %w", err)
	}
	return count, nil
}

// BulkCreate creates multiple tweets efficiently
func (r *TweetRepository) BulkCreate(ctx context.Context, tweets []struct {
	UserID  int64
	Content string
}) error {
	if len(tweets) == 0 {
		return nil
	}

	// Build bulk insert query
	valueStrings := make([]string, 0, len(tweets))
	valueArgs := make([]interface{}, 0, len(tweets)*2)
	
	for i, t := range tweets {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d)", i*2+1, i*2+2))
		valueArgs = append(valueArgs, t.UserID, t.Content)
	}

	query := fmt.Sprintf("INSERT INTO tweets (user_id, content) VALUES %s", strings.Join(valueStrings, ","))
	_, err := r.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		return fmt.Errorf("failed to bulk create tweets: %w", err)
	}
	return nil
}

// Delete deletes a tweet
func (r *TweetRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM tweets WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete tweet: %w", err)
	}
	return nil
}

// Truncate removes all tweets (for testing/reset)
func (r *TweetRepository) Truncate(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "TRUNCATE tweets CASCADE")
	if err != nil {
		return fmt.Errorf("failed to truncate tweets: %w", err)
	}
	return nil
}
