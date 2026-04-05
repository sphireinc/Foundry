package httpadmin

import (
	"net/http"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

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

func (r *Router) handleInstallTheme(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.ThemeInstallRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.InstallTheme(req.Context(), body.URL, body.Name, body.Kind)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	target := record.Name
	if record.Kind == "admin" {
		target = "admin:" + record.Name
	}
	r.logAuditRequest(req, "theme.install", "success", target, nil)
	writeJSON(w, http.StatusOK, record)
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
