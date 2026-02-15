package middleware

import (
	"context"
	"net/http"

	"linuxtorouter/internal/auth"
	"linuxtorouter/internal/models"
)

type contextKey string

const UserContextKey contextKey = "user"

type AuthMiddleware struct {
	sessions    *auth.SessionManager
	userService *auth.UserService
}

func NewAuthMiddleware(sessions *auth.SessionManager, userService *auth.UserService) *AuthMiddleware {
	return &AuthMiddleware{
		sessions:    sessions,
		userService: userService,
	}
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := m.sessions.GetUserID(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		user, err := m.userService.GetByID(userID)
		if err != nil {
			m.sessions.Clear(w, r)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil || !user.IsAdmin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func GetUser(r *http.Request) *models.User {
	user, _ := r.Context().Value(UserContextKey).(*models.User)
	return user
}
