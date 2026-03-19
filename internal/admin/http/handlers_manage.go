package httpadmin

import (
	"encoding/json"
	"net/http"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

func registerManagementRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: "/__admin/api/users", handler: http.HandlerFunc(r.handleUsers), role: "admin"},
		{pattern: "/__admin/api/users/save", handler: http.HandlerFunc(r.handleSaveUser), role: "admin"},
		{pattern: "/__admin/api/users/delete", handler: http.HandlerFunc(r.handleDeleteUser), role: "admin"},
		{pattern: "/__admin/api/config", handler: http.HandlerFunc(r.handleConfigDocument), role: "admin"},
		{pattern: "/__admin/api/config/save", handler: http.HandlerFunc(r.handleSaveConfigDocument), role: "admin"},
		{pattern: "/__admin/api/themes", handler: http.HandlerFunc(r.handleThemes), role: "admin"},
		{pattern: "/__admin/api/themes/switch", handler: http.HandlerFunc(r.handleThemeSwitch), role: "admin"},
		{pattern: "/__admin/api/plugins", handler: http.HandlerFunc(r.handlePlugins), role: "admin"},
		{pattern: "/__admin/api/plugins/enable", handler: http.HandlerFunc(r.handleEnablePlugin), role: "admin"},
		{pattern: "/__admin/api/plugins/disable", handler: http.HandlerFunc(r.handleDisablePlugin), role: "admin"},
	}
}

func (r *Router) handleUsers(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := r.service.ListUsers(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (r *Router) handleSaveUser(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.UserSaveRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	user, err := r.service.SaveUser(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (r *Router) handleDeleteUser(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.UserDeleteRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.service.DeleteUser(req.Context(), body.Username); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleConfigDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := r.service.LoadConfigDocument(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleSaveConfigDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.ConfigSaveRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := r.service.SaveConfigDocument(req.Context(), body.Raw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleThemes(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := r.service.ListThemes(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (r *Router) handleThemeSwitch(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.ThemeSwitchRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.service.SwitchTheme(req.Context(), body.Name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handlePlugins(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := r.service.ListPlugins(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (r *Router) handleEnablePlugin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PluginToggleRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.service.EnablePlugin(req.Context(), body.Name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleDisablePlugin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PluginToggleRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.service.DisablePlugin(req.Context(), body.Name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
