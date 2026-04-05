package httpadmin

import (
	"fmt"
	"net/http"
	"path/filepath"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
	"github.com/sphireinc/foundry/internal/safepath"
)

func (r *Router) handleBackups(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := r.service.ListBackups(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (r *Router) handleCreateBackup(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.BackupCreateRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.CreateBackup(req.Context(), body.Name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "backup.create", "success", record.Name, nil)
	writeJSON(w, http.StatusOK, record)
}

func (r *Router) handleRestoreBackup(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.BackupRestoreRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.RestoreBackup(req.Context(), body.Name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "backup.restore", "success", record.Name, nil)
	writeJSON(w, http.StatusOK, record)
}

func (r *Router) handleDownloadBackup(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	target, err := r.service.BackupPath(req.URL.Query().Get("name"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	ok, err := safepath.IsWithinRoot(r.cfg.Backup.Dir, target)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	if !ok {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("backup path is outside backup root"))
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(target)+"\"")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, req, target)
}

func (r *Router) handleGitBackups(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	items, err := r.service.ListGitBackupSnapshots(req.Context(), 20)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (r *Router) handleCreateGitBackup(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.BackupGitSnapshotRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	record, err := r.service.CreateGitBackupSnapshot(req.Context(), body.Message, body.Push)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "backup.git_snapshot.create", "success", record.Revision, nil)
	writeJSON(w, http.StatusOK, record)
}
