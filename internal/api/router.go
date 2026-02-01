package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// NewRouter creates and configures the HTTP router
func NewRouter(h *Handler) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	// CORS for web UI
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", h.HealthCheck)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Tweet operations
		r.Post("/tweet", h.PostTweet)

		// Timeline operations
		r.Get("/timeline/{user_id}", h.GetTimeline)

		// User operations
		r.Get("/users/sample", h.GetSampleUsers)
		r.Get("/users/{id}/followers", h.GetUserFollowers)
		r.Get("/users/{id}/following", h.GetUserFollowing)

		// Configuration
		r.Get("/config", h.GetConfig)
		r.Put("/config", h.UpdateConfig)

		// Metrics
		r.Get("/metrics", h.GetMetrics)
		r.Get("/metrics/recent", h.GetRecentMetrics)
		r.Delete("/metrics", h.ClearMetrics)
	})

	return r
}
