package httpadmin

import (
	"net/http"

	adminaudit "github.com/sphireinc/foundry/internal/admin/audit"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

// registerManagementRoutes returns admin configuration, users, themes, plugins,
// extensions, and audit routes.
func registerManagementRoutes(r *Router) []routeDef {
	return []routeDef{
		{pattern: r.routePath("/api/extensions"), handler: http.HandlerFunc(r.handleExtensions), capability: "dashboard.read"},
		{pattern: r.routePath("/api/settings/sections"), handler: http.HandlerFunc(r.handleSettingsSections), capability: "dashboard.read"},
		{pattern: r.routePath("/api/users"), handler: http.HandlerFunc(r.handleUsers), capability: "users.manage"},
		{pattern: r.routePath("/api/users/save"), handler: http.HandlerFunc(r.handleSaveUser), capability: "users.manage"},
		{pattern: r.routePath("/api/users/delete"), handler: http.HandlerFunc(r.handleDeleteUser), capability: "users.manage"},
		{pattern: r.routePath("/api/config"), handler: http.HandlerFunc(r.handleConfigDocument), capability: "config.manage"},
		{pattern: r.routePath("/api/config/save"), handler: http.HandlerFunc(r.handleSaveConfigDocument), capability: "config.manage"},
		{pattern: r.routePath("/api/themes"), handler: http.HandlerFunc(r.handleThemes), capability: "themes.manage"},
		{pattern: r.routePath("/api/themes/validate"), handler: http.HandlerFunc(r.handleValidateTheme), capability: "themes.manage"},
		{pattern: r.routePath("/api/themes/switch"), handler: http.HandlerFunc(r.handleThemeSwitch), capability: "themes.manage"},
		{pattern: r.routePath("/api/plugins"), handler: http.HandlerFunc(r.handlePlugins), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/validate"), handler: http.HandlerFunc(r.handleValidatePlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/install"), handler: http.HandlerFunc(r.handleInstallPlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/update"), handler: http.HandlerFunc(r.handleUpdatePlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/rollback"), handler: http.HandlerFunc(r.handleRollbackPlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/enable"), handler: http.HandlerFunc(r.handleEnablePlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/plugins/disable"), handler: http.HandlerFunc(r.handleDisablePlugin), capability: "plugins.manage"},
		{pattern: r.routePath("/api/audit"), handler: http.HandlerFunc(r.handleAudit), capability: "audit.read"},
	}
}

func (r *Router) handleExtensions(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	registry, err := r.service.ListAdminExtensions(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, registry)
}

func (r *Router) handleSettingsSections(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sections, err := r.service.ListSettingsSections(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, sections)
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
	if err := decodeJSONBody(w, req, mediumJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	user, err := r.service.SaveUser(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "user.save", "success", user.Username, nil)
	writeJSON(w, http.StatusOK, user)
}

func (r *Router) handleDeleteUser(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.UserDeleteRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	if err := r.service.DeleteUser(req.Context(), body.Username); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "user.delete", "success", body.Username, nil)
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
	if err := decodeJSONBody(w, req, configJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.SaveConfigDocument(req.Context(), body.Raw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "config.save", "success", resp.Path, nil)
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
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	var err error
	if body.Kind == "admin" {
		err = r.service.SwitchAdminTheme(req.Context(), body.Name)
	} else {
		err = r.service.SwitchTheme(req.Context(), body.Name)
	}
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	target := body.Name
	if body.Kind == "admin" {
		target = "admin:" + body.Name
	}
	r.logAuditRequest(req, "theme.switch", "success", target, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleValidateTheme(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.ThemeSwitchRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.ValidateTheme(req.Context(), body.Name, body.Kind)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	target := body.Name
	if body.Kind == "admin" {
		target = "admin:" + body.Name
	}
	r.logAuditRequest(req, "theme.validate", "success", target, nil)
	writeJSON(w, http.StatusOK, record)
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
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	if err := r.service.EnablePlugin(req.Context(), body.Name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "plugin.enable", "success", body.Name, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleInstallPlugin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PluginInstallRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.InstallPlugin(req.Context(), body.URL, body.Name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "plugin.install", "success", record.Name, nil)
	writeJSON(w, http.StatusOK, record)
}

func (r *Router) handleValidatePlugin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PluginToggleRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.ValidatePlugin(req.Context(), body.Name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "plugin.validate", "success", record.Name, nil)
	writeJSON(w, http.StatusOK, record)
}

func (r *Router) handleUpdatePlugin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PluginToggleRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.UpdatePlugin(req.Context(), body.Name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "plugin.update", "success", record.Name, nil)
	writeJSON(w, http.StatusOK, record)
}

func (r *Router) handleRollbackPlugin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PluginToggleRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.RollbackPlugin(req.Context(), body.Name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "plugin.rollback", "success", record.Name, nil)
	writeJSON(w, http.StatusOK, record)
}

func (r *Router) handleDisablePlugin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.PluginToggleRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	if err := r.service.DisablePlugin(req.Context(), body.Name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "plugin.disable", "success", body.Name, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleAudit(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := adminaudit.List(r.cfg, 200)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
