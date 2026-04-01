package modernize

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

const maxToolIterations = 25

// ChatHandler creates a Gin handler for the chat endpoint.
// This is HTTP glue that delegates to RunChat.
func ChatHandler(ps *ProviderState, mcpClient *MCPClient, defaultModel string, defaultMaxTokens int, sessionStore SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ps.Get() == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		var req struct {
			Messages       []chatInputMessage `json:"messages"`
			TargetLanguage string             `json:"targetLanguage"`
			Framework      string             `json:"framework"`
			Integrations   string             `json:"integrations"`
			DiscoveryMode  bool               `json:"discoveryMode"`
			SessionID      string             `json:"sessionId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		SetSSEHeaders(c.Writer)

		emitter := &SSEEmitter{W: c.Writer, Mu: &sync.Mutex{}}

		err := RunChat(c.Request.Context(), ChatParams{
			Provider:      ps,
			MCPClient:     mcpClient,
			SessionStore:  sessionStore,
			SessionID:     req.SessionID,
			Messages:      req.Messages,
			DiscoveryMode: req.DiscoveryMode,
			TargetLang:    req.TargetLanguage,
			Framework:     req.Framework,
			Integrations:  req.Integrations,
			Emitter:       emitter,
			Logger:        nil,
		})
		if err != nil {
			emitter.Emit("error", map[string]string{"error": err.Error()})
		}
	}
}

type chatInputMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
