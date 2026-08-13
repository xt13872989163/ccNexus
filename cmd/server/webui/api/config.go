package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/lich0821/ccNexus/internal/config"
	"github.com/lich0821/ccNexus/internal/logger"
)

var errBasicAuthCredentialsRequired = errors.New("username and password are required when Basic Auth is enabled")

type BasicAuthConfigRequest struct {
	Enabled  bool   `json:"enabled"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleConfig handles GET and PUT for full configuration
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r)
	case http.MethodPut:
		h.updateConfig(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) handleBasicAuthConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		enabled, username, password := h.config.GetBasicAuth()
		WriteSuccess(w, map[string]interface{}{
			"enabled":     enabled,
			"username":    username,
			"hasPassword": password != "",
		})
	case http.MethodPut:
		var req BasicAuthConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if err := h.persistBasicAuth(&req.Enabled, req.Username, req.Password); err != nil {
			if errors.Is(err, errBasicAuthCredentialsRequired) {
				WriteError(w, http.StatusBadRequest, err.Error())
				return
			}
			logger.Error("Failed to save config: %v", err)
			WriteError(w, http.StatusInternalServerError, "Failed to save configuration")
			return
		}

		WriteSuccess(w, map[string]interface{}{
			"message":  "Basic Auth configuration updated",
			"enabled":  h.config.GetBasicAuthEnabled(),
			"username": h.config.GetBasicAuthUsername(),
		})
	default:
		WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) handleResetBasicAuthPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to generate password")
		return
	}
	newPassword := hex.EncodeToString(bytes)[:16]

	if err := h.persistBasicAuth(nil, "", newPassword); err != nil {
		logger.Error("Failed to save config: %v", err)
		WriteError(w, http.StatusInternalServerError, "Failed to save configuration")
		return
	}

	logger.Info("Basic Auth password has been reset via API")

	WriteSuccess(w, map[string]interface{}{
		"message":  "Password reset successfully",
		"password": newPassword,
	})
}

// getConfig returns the full configuration
func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	WriteSuccess(w, map[string]interface{}{
		"port":         h.config.GetPort(),
		"portBindings": h.config.GetPortBindings(),
		"logLevel":     h.config.GetLogLevel(),
	})
}

// updateConfig updates the full configuration
func (h *Handler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Port         *int                 `json:"port"`
		PortBindings *[]config.PortBinding `json:"portBindings"`
		LogLevel     *int                 `json:"logLevel"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Port != nil {
		if h.config.IsPortLocked() {
			WriteError(w, http.StatusForbidden, "Port is locked by CLI flag and cannot be changed")
			return
		}
		if *req.Port < 1 || *req.Port > 65535 {
			WriteError(w, http.StatusBadRequest, "Invalid port number")
			return
		}
	}
	if req.LogLevel != nil && (*req.LogLevel < 0 || *req.LogLevel > 3) {
		WriteError(w, http.StatusBadRequest, "Invalid log level (must be 0-3)")
		return
	}
	if req.PortBindings != nil {
		candidate := config.DefaultConfig()
		candidate.Port = h.config.GetPort()
		candidate.Endpoints = h.config.GetEndpoints()
		candidate.PortBindings = *req.PortBindings
		if err := candidate.Validate(); err != nil {
			WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := h.persistRuntimeConfig(req.Port, req.LogLevel, req.PortBindings); err != nil {
		logger.Error("Failed to save config: %v", err)
		WriteError(w, http.StatusInternalServerError, "Failed to save configuration")
		return
	}
	if req.LogLevel != nil {
		logger.GetLogger().SetMinLevel(logger.LogLevel(*req.LogLevel))
		logger.GetLogger().SetConsoleLevel(logger.LogLevel(*req.LogLevel))
	}

	WriteSuccess(w, map[string]interface{}{
		"message": "Configuration updated successfully",
	})
}

// handleConfigPort handles GET and PUT for port configuration
func (h *Handler) handleConfigPort(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		WriteSuccess(w, map[string]interface{}{
			"port":       h.config.GetPort(),
			"portLocked": h.config.IsPortLocked(),
		})
	case http.MethodPut:
		if h.config.IsPortLocked() {
			WriteError(w, http.StatusForbidden, "Port is locked by CLI flag and cannot be changed")
			return
		}

		var req struct {
			Port int `json:"port"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.Port <= 0 || req.Port > 65535 {
			WriteError(w, http.StatusBadRequest, "Invalid port number")
			return
		}

		if err := h.persistRuntimeConfig(&req.Port, nil, nil); err != nil {
			logger.Error("Failed to save config: %v", err)
			WriteError(w, http.StatusInternalServerError, "Failed to save configuration")
			return
		}

		WriteSuccess(w, map[string]interface{}{
			"port":    req.Port,
			"message": "Port updated successfully (restart required)",
		})
	default:
		WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// handleConfigLogLevel handles GET and PUT for log level configuration
func (h *Handler) handleConfigLogLevel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		WriteSuccess(w, map[string]interface{}{
			"logLevel": h.config.GetLogLevel(),
		})
	case http.MethodPut:
		var req struct {
			LogLevel int `json:"logLevel"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if req.LogLevel < 0 || req.LogLevel > 3 {
			WriteError(w, http.StatusBadRequest, "Invalid log level (must be 0-3)")
			return
		}

		if err := h.persistRuntimeConfig(nil, &req.LogLevel, nil); err != nil {
			logger.Error("Failed to save config: %v", err)
			WriteError(w, http.StatusInternalServerError, "Failed to save configuration")
			return
		}

		logger.GetLogger().SetMinLevel(logger.LogLevel(req.LogLevel))
		logger.GetLogger().SetConsoleLevel(logger.LogLevel(req.LogLevel))

		WriteSuccess(w, map[string]interface{}{
			"logLevel": req.LogLevel,
			"message":  "Log level updated successfully",
		})
	default:
		WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (h *Handler) persistBasicAuth(enabled *bool, username, password string) error {
	h.configMu.Lock()
	defer h.configMu.Unlock()

	oldEnabled := h.config.GetBasicAuthEnabled()
	oldUsername := h.config.GetBasicAuthUsername()
	oldPassword := h.config.GetBasicAuthPassword()
	newEnabled := oldEnabled
	if enabled != nil {
		newEnabled = *enabled
	}
	if username == "" {
		username = oldUsername
	}
	if password == "" || password == "***" {
		password = oldPassword
	}
	username = strings.TrimSpace(username)
	if newEnabled && (username == "" || password == "") {
		return errBasicAuthCredentialsRequired
	}

	h.config.UpdateBasicAuth(newEnabled, username, password)
	if err := h.storage.SetConfigs(map[string]string{
		"basicAuthEnabled":  strconv.FormatBool(newEnabled),
		"basicAuthUsername": username,
		"basicAuthPassword": password,
	}); err != nil {
		h.config.UpdateBasicAuth(oldEnabled, oldUsername, oldPassword)
		return err
	}
	return nil
}

func (h *Handler) persistRuntimeConfig(port, logLevel *int, bindings *[]config.PortBinding) error {
	h.configMu.Lock()
	defer h.configMu.Unlock()

	oldPort := h.config.GetPort()
	oldLogLevel := h.config.GetLogLevel()
	oldBindings := h.config.GetPortBindings()
	if port != nil {
		h.config.UpdatePort(*port)
	}
	if logLevel != nil {
		h.config.UpdateLogLevel(*logLevel)
	}
	if bindings != nil {
		h.config.PortBindings = append([]config.PortBinding(nil), (*bindings)...)
	}

	values := make(map[string]string, 3)
	if port != nil {
		values["port"] = strconv.Itoa(*port)
	}
	if logLevel != nil {
		values["logLevel"] = strconv.Itoa(*logLevel)
	}
	if bindings != nil {
		encoded, err := json.Marshal(*bindings)
		if err != nil {
			return err
		}
		values["portBindings"] = string(encoded)
	}
	if err := h.storage.SetConfigs(values); err != nil {
		h.config.UpdatePort(oldPort)
		h.config.UpdateLogLevel(oldLogLevel)
		h.config.PortBindings = oldBindings
		return err
	}
	return nil
}
