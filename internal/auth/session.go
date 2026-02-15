package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const (
	SessionName    = "router-session"
	SessionUserID  = "user_id"
	SessionIsAdmin = "is_admin"
)

type SessionManager struct {
	store *sessions.CookieStore
}

func NewSessionManager(secret string, maxAge int) *SessionManager {
	store := sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	}
	return &SessionManager{store: store}
}

func (m *SessionManager) Get(r *http.Request) (*sessions.Session, error) {
	return m.store.Get(r, SessionName)
}

func (m *SessionManager) SetUser(w http.ResponseWriter, r *http.Request, userID int64, isAdmin bool, remember bool) error {
	session, err := m.Get(r)
	if err != nil {
		return err
	}

	session.Values[SessionUserID] = userID
	session.Values[SessionIsAdmin] = isAdmin

	if remember {
		session.Options.MaxAge = 86400 * 30 // 30 days
	}

	return session.Save(r, w)
}

func (m *SessionManager) GetUserID(r *http.Request) (int64, bool) {
	session, err := m.Get(r)
	if err != nil {
		return 0, false
	}

	userID, ok := session.Values[SessionUserID].(int64)
	return userID, ok
}

func (m *SessionManager) IsAdmin(r *http.Request) bool {
	session, err := m.Get(r)
	if err != nil {
		return false
	}

	isAdmin, ok := session.Values[SessionIsAdmin].(bool)
	return ok && isAdmin
}

func (m *SessionManager) Clear(w http.ResponseWriter, r *http.Request) error {
	session, err := m.Get(r)
	if err != nil {
		return err
	}

	session.Values = make(map[interface{}]interface{})
	session.Options.MaxAge = -1

	return session.Save(r, w)
}
