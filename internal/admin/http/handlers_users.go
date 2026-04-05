package httpadmin

import (
	"net/http"
	"strings"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/admin/users"
)

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
	var existing *users.User
	if all, loadErr := users.Load(r.cfg.Admin.UsersFile); loadErr == nil {
		for i := range all {
			if strings.EqualFold(strings.TrimSpace(all[i].Username), strings.TrimSpace(body.Username)) {
				copyUser := all[i]
				existing = &copyUser
				break
			}
		}
	}
	user, err := r.service.SaveUser(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	metadata := map[string]string{
		"new_user": boolString(existing == nil),
	}
	if strings.TrimSpace(body.Password) != "" {
		metadata["password_changed"] = "true"
		metadata["sessions_revoked"] = "true"
	}
	if existing != nil {
		if strings.TrimSpace(existing.Role) != strings.TrimSpace(user.Role) {
			metadata["role_changed"] = "true"
			metadata["sessions_revoked"] = "true"
		}
		if existing.Disabled != user.Disabled {
			metadata["disabled_changed"] = "true"
			metadata["sessions_revoked"] = "true"
		}
		if !stringSliceSetEqual(existing.Capabilities, user.Capabilities) {
			metadata["capabilities_changed"] = "true"
			metadata["sessions_revoked"] = "true"
		}
	}
	r.logAuditRequest(req, "user.save", "success", user.Username, metadata)
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
	r.logAuditRequest(req, "user.delete", "success", body.Username, map[string]string{"sessions_revoked": "true"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func stringSliceSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]int, len(a))
	for _, item := range a {
		set[strings.TrimSpace(item)]++
	}
	for _, item := range b {
		trimmed := strings.TrimSpace(item)
		if set[trimmed] == 0 {
			return false
		}
		set[trimmed]--
	}
	for _, count := range set {
		if count != 0 {
			return false
		}
	}
	return true
}
