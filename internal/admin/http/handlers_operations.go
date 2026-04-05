package httpadmin

import (
	"net/http"

	"github.com/sphireinc/foundry/internal/logx"
)

func (r *Router) handleUpdateStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := r.service.CheckForUpdates(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleOperationsStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := r.service.GetOperationsStatus(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleOperationsLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := r.service.ReadOperationsLog(req.Context(), 120)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleOperationsValidate(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := r.service.ValidateSite(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (r *Router) handleOperationsClearCache(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.service.ClearOperationalCaches(req.Context()); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	r.logAuditRequest(req, "operations.cache.clear", "success", "graph-cache", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleOperationsRebuild(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.service.RebuildSite(req.Context()); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "operations.rebuild", "success", "build", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleApplyUpdate(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	logx.Info("admin api update apply started", "path", req.URL.Path)
	resp, err := r.service.ApplyUpdate(req.Context())
	if err != nil {
		logx.Error("admin api update apply failed", "path", req.URL.Path, "error", err)
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	logx.Info("admin api update apply accepted", "latest_version", resp.LatestVersion, "install_mode", resp.InstallMode, "apply_supported", resp.ApplySupported)
	r.logAuditRequest(req, "system.update.apply", "success", resp.LatestVersion, nil)
	writeJSON(w, http.StatusAccepted, resp)
}
