package modernize

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Session represents a chat session with its full message history.
type Session struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Messages  []chatInputMessage `json:"messages"`
	CreatedAt time.Time          `json:"createdAt"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

// SessionSummary is a lightweight view for listing sessions.
type SessionSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SessionStore abstracts session persistence.
// SQLite implementation now; Azure Table Storage / PostgreSQL later.
type SessionStore interface {
	List() ([]SessionSummary, error)
	Get(id string) (*Session, error)
	Create(title string) (*Session, error)
	Update(id string, messages []chatInputMessage, title string) error
	Delete(id string) error
	Close() error
}

// ListSessionsHandler returns all sessions ordered by most recently updated.
func ListSessionsHandler(store SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessions, err := store.List()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sessions)
	}
}

// GetSessionHandler returns a single session with its full message history.
func GetSessionHandler(store SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		session, err := store.Get(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		c.JSON(http.StatusOK, session)
	}
}

// CreateSessionHandler creates a new session with an optional title.
func CreateSessionHandler(store SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			req.Title = "New Chat"
		}
		if req.Title == "" {
			req.Title = "New Chat"
		}
		session, err := store.Create(req.Title)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, session)
	}
}

// UpdateSessionHandler renames a session.
func UpdateSessionHandler(store SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req struct {
			Title string `json:"title"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := store.Update(id, nil, req.Title); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

// DeleteSessionHandler deletes a session and its messages.
func DeleteSessionHandler(store SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := store.Delete(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
