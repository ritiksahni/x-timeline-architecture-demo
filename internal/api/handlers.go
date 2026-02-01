package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ritik/twitter-fan-out/internal/config"
	"github.com/ritik/twitter-fan-out/internal/models"
	"github.com/ritik/twitter-fan-out/internal/repository"
	"github.com/ritik/twitter-fan-out/internal/timeline"
)

// Handler holds all HTTP handlers
type Handler struct {
	config         *config.Config
	fanOutWrite    *timeline.FanOutWriteStrategy
	fanOutRead     *timeline.FanOutReadStrategy
	hybrid         *timeline.HybridStrategy
	metricsStore   *MetricsStore
	userRepo       *repository.UserRepository
	followRepo     *repository.FollowRepository
}

// NewHandler creates a new Handler
func NewHandler(
	cfg *config.Config,
	fanOutWrite *timeline.FanOutWriteStrategy,
	fanOutRead *timeline.FanOutReadStrategy,
	hybrid *timeline.HybridStrategy,
	userRepo *repository.UserRepository,
	followRepo *repository.FollowRepository,
) *Handler {
	return &Handler{
		config:       cfg,
		fanOutWrite:  fanOutWrite,
		fanOutRead:   fanOutRead,
		hybrid:       hybrid,
		metricsStore: NewMetricsStore(),
		userRepo:     userRepo,
		followRepo:   followRepo,
	}
}

// Response helpers
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// PostTweetRequest represents the request body for posting a tweet
type PostTweetRequest struct {
	UserID   int64  `json:"user_id"`
	Content  string `json:"content"`
	Strategy string `json:"strategy"`
}

// PostTweet handles POST /api/tweet
func (h *Handler) PostTweet(w http.ResponseWriter, r *http.Request) {
	var req PostTweetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.UserID == 0 {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Content == "" {
		respondError(w, http.StatusBadRequest, "content is required")
		return
	}
	if req.Strategy == "" {
		req.Strategy = "hybrid" // Default strategy
	}

	var tweet *models.Tweet
	var metrics *timeline.OperationMetrics
	var err error

	ctx := r.Context()

	switch req.Strategy {
	case "fanout_write":
		tweet, metrics, err = h.fanOutWrite.PostTweet(ctx, req.UserID, req.Content)
	case "fanout_read":
		tweet, metrics, err = h.fanOutRead.PostTweet(ctx, req.UserID, req.Content)
	case "hybrid":
		tweet, metrics, err = h.hybrid.PostTweet(ctx, req.UserID, req.Content)
	default:
		respondError(w, http.StatusBadRequest, "Invalid strategy. Use: fanout_write, fanout_read, or hybrid")
		return
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Store metrics
	h.metricsStore.AddWriteMetric(metrics)

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"tweet":   tweet,
		"metrics": metricsToJSON(metrics),
	})
}

// GetTimeline handles GET /api/timeline/{user_id}
func (h *Handler) GetTimeline(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}

	strategy := r.URL.Query().Get("strategy")
	if strategy == "" {
		strategy = "hybrid"
	}

	limitStr := r.URL.Query().Get("limit")
	limit := h.config.TimelinePageSize
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offsetStr := r.URL.Query().Get("offset")
	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var tweets []*models.Tweet
	var metrics *timeline.OperationMetrics

	ctx := r.Context()

	switch strategy {
	case "fanout_write":
		tweets, metrics, err = h.fanOutWrite.GetTimeline(ctx, userID, limit, offset)
	case "fanout_read":
		tweets, metrics, err = h.fanOutRead.GetTimeline(ctx, userID, limit, offset)
	case "hybrid":
		tweets, metrics, err = h.hybrid.GetTimeline(ctx, userID, limit, offset)
	default:
		respondError(w, http.StatusBadRequest, "Invalid strategy. Use: fanout_write, fanout_read, or hybrid")
		return
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Store metrics
	h.metricsStore.AddReadMetric(metrics)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":  userID,
		"tweets":   tweets,
		"count":    len(tweets),
		"limit":    limit,
		"offset":   offset,
		"strategy": strategy,
		"metrics":  metricsToJSON(metrics),
	})
}

// GetConfig handles GET /api/config
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"celebrity_threshold":  h.config.CelebrityThreshold,
		"timeline_cache_size":  h.config.TimelineCacheSize,
		"timeline_page_size":   h.config.TimelinePageSize,
	})
}

// UpdateConfigRequest represents the request body for updating config
type UpdateConfigRequest struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// UpdateConfig handles PUT /api/config
func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Convert value to int if it's a float64 (JSON numbers)
	if v, ok := req.Value.(float64); ok {
		req.Value = int(v)
	}

	h.config.Update(req.Key, req.Value)

	// Update hybrid strategy threshold if needed
	if req.Key == "celebrity_threshold" || req.Key == "celebrity-threshold" {
		if v, ok := req.Value.(int); ok {
			h.hybrid.SetCelebrityThreshold(v)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Config updated",
		"key":     req.Key,
		"value":   req.Value,
	})
}

// GetMetrics handles GET /api/metrics
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.metricsStore.GetSummary()
	respondJSON(w, http.StatusOK, metrics)
}

// GetRecentMetrics handles GET /api/metrics/recent
func (h *Handler) GetRecentMetrics(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	metrics := h.metricsStore.GetRecent(limit)
	respondJSON(w, http.StatusOK, metrics)
}

// ClearMetrics handles DELETE /api/metrics
func (h *Handler) ClearMetrics(w http.ResponseWriter, r *http.Request) {
	h.metricsStore.Clear()
	respondJSON(w, http.StatusOK, map[string]string{"message": "Metrics cleared"})
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetSampleUsers handles GET /api/users/sample
func (h *Handler) GetSampleUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Get a mix of regular users and celebrities
	regularUsers, err := h.userRepo.GetRandomUsers(ctx, 3)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	celebrities, err := h.userRepo.GetCelebrities(ctx, h.config.CelebrityThreshold)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	// Take first celebrity if available
	sampleCelebrity := []*models.User{}
	if len(celebrities) > 0 {
		sampleCelebrity = []*models.User{celebrities[0]}
	}
	
	// Combine and format
	allUsers := append(regularUsers, sampleCelebrity...)
	users := make([]map[string]interface{}, len(allUsers))
	for i, user := range allUsers {
		users[i] = map[string]interface{}{
			"id":             user.ID,
			"username":      user.Username,
			"follower_count": user.FollowerCount,
			"is_celebrity":   user.IsCelebrity(h.config.CelebrityThreshold),
		}
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"users": users,
	})
}

// GetUserFollowers handles GET /api/users/{id}/followers
func (h *Handler) GetUserFollowers(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}
	
	ctx := r.Context()
	
	// Verify user exists
	_, err = h.userRepo.GetByID(ctx, userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}
	
	// Get followers
	followers, err := h.followRepo.GetFollowers(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	// Return first 3 as sample
	sampleFollowers := followers
	if len(sampleFollowers) > 3 {
		sampleFollowers = sampleFollowers[:3]
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":         userID,
		"follower_count":  len(followers),
		"sample_followers": sampleFollowers,
	})
}

// GetUserFollowing handles GET /api/users/{id}/following
func (h *Handler) GetUserFollowing(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user_id")
		return
	}
	
	ctx := r.Context()
	
	// Get user info
	_, err = h.userRepo.GetByID(ctx, userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}
	
	// Get following
	following, err := h.followRepo.GetFollowing(ctx, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":         userID,
		"following_count": len(following),
		"following":       following,
	})
}

// Helper to convert metrics to JSON-friendly format
func metricsToJSON(m *timeline.OperationMetrics) map[string]interface{} {
	result := map[string]interface{}{
		"strategy":      m.Strategy,
		"operation":     m.Operation,
		"duration_ms":   m.Duration().Milliseconds(),
		"duration":      m.Duration().String(),
		"success":       m.Success,
		"cache_hit":     m.CacheHit,
		"fan_out_count": m.FanOutCount, // Always include, even if 0
	}

	if m.FanOutDuration > 0 {
		result["fan_out_duration_ms"] = m.FanOutDuration.Milliseconds()
		result["fan_out_duration"] = m.FanOutDuration.String()
	}
	if m.Error != nil {
		result["error"] = m.Error.Error()
	}

	return result
}
