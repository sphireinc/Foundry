package httpadmin

import (
	"net/http"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

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
	if err := r.service.EnablePlugin(req.Context(), body.Name, body.ApproveRisk, body.AcknowledgeMismatches); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	metadata := map[string]string{}
	if body.ApproveRisk {
		metadata["approve_risk"] = "true"
	}
	if body.AcknowledgeMismatches {
		metadata["acknowledge_mismatches"] = "true"
	}
	r.logAuditRequest(req, "plugin.enable", "success", body.Name, metadata)
	if body.ApproveRisk || body.AcknowledgeMismatches {
		r.logPluginSecurityAudit(req, "plugin.enable.approved", body.Name)
	}
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
	record, err := r.service.InstallPlugin(req.Context(), body.URL, body.Name, body.ApproveRisk, body.AcknowledgeMismatches)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	metadata := map[string]string{}
	if body.ApproveRisk {
		metadata["approve_risk"] = "true"
	}
	if body.AcknowledgeMismatches {
		metadata["acknowledge_mismatches"] = "true"
	}
	r.logAuditRequest(req, "plugin.install", "success", record.Name, metadata)
	if body.ApproveRisk || body.AcknowledgeMismatches {
		r.logPluginSecurityAudit(req, "plugin.install.approved", record.Name)
	}
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
	record, err := r.service.UpdatePlugin(req.Context(), body.Name, body.ApproveRisk, body.AcknowledgeMismatches)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	metadata := map[string]string{}
	if body.ApproveRisk {
		metadata["approve_risk"] = "true"
	}
	if body.AcknowledgeMismatches {
		metadata["acknowledge_mismatches"] = "true"
	}
	r.logAuditRequest(req, "plugin.update", "success", record.Name, metadata)
	if body.ApproveRisk || body.AcknowledgeMismatches {
		r.logPluginSecurityAudit(req, "plugin.update.approved", record.Name)
	}
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
