package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"llm-gateway/config"
	"llm-gateway/models"

	"gorm.io/gorm"
)

type SessionHandler struct {
	db *gorm.DB
}

func NewSessionHandler(db *gorm.DB) *SessionHandler {
	return &SessionHandler{db: db}
}

var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var sessionIDRegex = regexp.MustCompile(`^.{1,64}$`)
var projectIDRegex = regexp.MustCompile(`^.{1,64}$`)
var engineRegex = regexp.MustCompile(`^.{1,128}$`)

func isValidUUID(text string) bool {
	return uuidRegex.MatchString(text)
}

func isValidSessionID(text string) bool {
	return sessionIDRegex.MatchString(text)
}

func isValidProjectID(text string) bool {
	if text == "" {
		return true
	}
	return projectIDRegex.MatchString(text)
}

func isValidEngine(text string) bool {
	if text == "" {
		return true
	}
	return engineRegex.MatchString(text)
}

func (h *SessionHandler) auth(w http.ResponseWriter, r *http.Request) bool {
	sessionServerKey := config.AppConfig.SessionServerKey
	if sessionServerKey == "" {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	expected := "Bearer " + sessionServerKey
	if authHeader != expected {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

type sessionPayload struct {
	ProjectID string      `json:"project_id"`
	AgentID   string      `json:"agent_id"`
	Engine    string      `json:"engine"`
	SessionID string      `json:"session_id"`
	Events    interface{} `json:"events"`
}

func (h *SessionHandler) validateSessionPayload(pathSessionID string, body *sessionPayload) error {
	if body == nil {
		return errors.New("empty body")
	}

	if body.SessionID == "" {
		return errors.New("session_id is required")
	}

	if body.SessionID != pathSessionID {
		return errors.New("session_id mismatch")
	}

	if !isValidProjectID(body.ProjectID) {
		return errors.New("invalid project_id")
	}

	if !isValidEngine(body.Engine) {
		return errors.New("invalid engine")
	}

	if !isValidSessionID(body.SessionID) {
		return errors.New("invalid session_id")
	}

	events, ok := body.Events.([]interface{})
	if !ok {
		return errors.New("events must be array")
	}

	for _, event := range events {
		if _, ok := event.(map[string]interface{}); !ok {
			return errors.New("invalid event item")
		}
	}

	return nil
}

func (h *SessionHandler) UploadSession(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}

	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" || !isValidSessionID(sessionID) {
		http.Error(w, `{"error": "invalid session_id"}`, http.StatusBadRequest)
		return
	}

	if r.Body == nil {
		http.Error(w, `{"error": "empty body"}`, http.StatusBadRequest)
		return
	}

	var body sessionPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error": "empty body"}`, http.StatusBadRequest)
		return
	}

	if err := h.validateSessionPayload(sessionID, &body); err != nil {
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	record := models.Session{
		ProjectID: body.ProjectID,
		AgentID:   body.AgentID,
		Engine:    body.Engine,
		SessionID: body.SessionID,
		Events:    models.JSONSlice(body.Events.([]interface{})),
	}

	result := h.db.Save(&record)
	if result.Error != nil {
		http.Error(w, `{"error": "failed to save session"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"project_id": body.ProjectID,
		"agent_id":   body.AgentID,
		"engine":     body.Engine,
		"ok":         true,
		"session_id": sessionID,
	})
}

func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	if !h.auth(w, r) {
		return
	}

	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" || !isValidSessionID(sessionID) {
		http.Error(w, `{"error": "invalid session_id"}`, http.StatusBadRequest)
		return
	}

	var record models.Session
	result := h.db.Where("session_id = ?", sessionID).First(&record)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			http.Error(w, `{"error": "not found"}`, http.StatusNotFound)
			return
		}
		http.Error(w, `{"error": "database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"project_id": record.ProjectID,
		"agent_id":   record.AgentID,
		"engine":     record.Engine,
		"session_id": record.SessionID,
		"events":     record.Events,
	})
}

func extractSessionID(path string) string {
	parts := regexp.MustCompile(`/sessions/([^/]+)`).FindStringSubmatch(path)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

func RegisterSessionRoutes(mux *http.ServeMux, db *gorm.DB) {
	handler := NewSessionHandler(db)
	mux.HandleFunc("/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.UploadSession(w, r)
		} else if r.Method == http.MethodGet {
			handler.GetSession(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
