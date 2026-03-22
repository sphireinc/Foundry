package httpadmin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	adminauth "github.com/sphireinc/foundry/internal/admin/auth"
	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

const adminMediaUploadLimit = 256 << 20

func registerDocumentRoutes(r *Router) []routeDef {
	return []routeDef{
		{
			pattern:    r.routePath("/api/documents"),
			handler:    http.HandlerFunc(r.handleDocuments),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/document"),
			handler:    http.HandlerFunc(r.handleDocument),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/create"),
			handler:    http.HandlerFunc(r.handleCreateDocument),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/save"),
			handler:    http.HandlerFunc(r.handleSaveDocument),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/lock"),
			handler:    http.HandlerFunc(r.handleDocumentLock),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/lock/heartbeat"),
			handler:    http.HandlerFunc(r.handleDocumentLockHeartbeat),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/unlock"),
			handler:    http.HandlerFunc(r.handleDocumentUnlock),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/history"),
			handler:    http.HandlerFunc(r.handleDocumentHistory),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/trash"),
			handler:    http.HandlerFunc(r.handleDocumentTrash),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/restore"),
			handler:    http.HandlerFunc(r.handleRestoreDocument),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/purge"),
			handler:    http.HandlerFunc(r.handlePurgeDocument),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/diff"),
			handler:    http.HandlerFunc(r.handleDocumentDiff),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/status"),
			handler:    http.HandlerFunc(r.handleDocumentStatus),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/delete"),
			handler:    http.HandlerFunc(r.handleDeleteDocument),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/documents/preview"),
			handler:    http.HandlerFunc(r.handlePreviewDocument),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media"),
			handler:    http.HandlerFunc(r.handleMedia),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/detail"),
			handler:    http.HandlerFunc(r.handleMediaDetail),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/history"),
			handler:    http.HandlerFunc(r.handleMediaHistory),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/trash"),
			handler:    http.HandlerFunc(r.handleMediaTrash),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/upload"),
			handler:    http.HandlerFunc(r.handleMediaUpload),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/replace"),
			handler:    http.HandlerFunc(r.handleMediaReplace),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/metadata"),
			handler:    http.HandlerFunc(r.handleMediaMetadata),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/delete"),
			handler:    http.HandlerFunc(r.handleMediaDelete),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/restore"),
			handler:    http.HandlerFunc(r.handleMediaRestore),
			capability: "dashboard.read",
		},
		{
			pattern:    r.routePath("/api/media/purge"),
			handler:    http.HandlerFunc(r.handleMediaPurge),
			capability: "dashboard.read",
		},
	}
}

func (r *Router) handleDocuments(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opts := admintypes.DocumentListOptions{
		IncludeDrafts: truthy(req.URL.Query().Get("include_drafts")),
		Type:          strings.TrimSpace(req.URL.Query().Get("type")),
		Lang:          strings.TrimSpace(req.URL.Query().Get("lang")),
		Query:         strings.TrimSpace(req.URL.Query().Get("q")),
	}

	docs, err := r.service.ListDocuments(req.Context(), opts)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, docs)
}

func (r *Router) handleDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(req.URL.Query().Get("id"))
	if id == "" {
		writeJSONErrorMessage(w, http.StatusBadRequest, "missing required query parameter: id")
		return
	}

	includeDrafts := truthy(req.URL.Query().Get("include_drafts"))
	doc, err := r.service.GetDocument(req.Context(), id, includeDrafts)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (r *Router) handleSaveDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.DocumentSaveRequest
	if err := decodeJSONBody(w, req, largeJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	body.Actor = actorLabel(req)
	body.Username = actorUsername(req)

	resp, err := r.service.SaveDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "document.save", "success", resp.SourcePath, nil)

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleDocumentLock(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.DocumentLockRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.AcquireDocumentLock(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleDocumentLockHeartbeat(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.DocumentLockRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.HeartbeatDocumentLock(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleDocumentUnlock(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.DocumentLockRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	if err := r.service.ReleaseDocumentLock(req.Context(), body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, admintypes.DocumentLockResponse{})
}

func (r *Router) handleDocumentHistory(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sourcePath := strings.TrimSpace(req.URL.Query().Get("source_path"))
	if sourcePath == "" {
		writeJSONErrorMessage(w, http.StatusBadRequest, "missing required query parameter: source_path")
		return
	}
	resp, err := r.service.GetDocumentHistory(req.Context(), sourcePath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleDocumentTrash(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := r.service.ListDocumentTrash(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleRestoreDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.DocumentLifecycleRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.RestoreDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "document.restore", "success", firstNonEmpty(resp.RestoredPath, resp.Path), nil)
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handlePurgeDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.DocumentLifecycleRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.PurgeDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "document.purge", "success", resp.Path, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleDocumentDiff(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.DocumentDiffRequest
	if err := decodeJSONBody(w, req, mediumJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.DiffDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleCreateDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.DocumentCreateRequest
	if err := decodeJSONBody(w, req, mediumJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}

	resp, err := r.service.CreateDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "document.create", "success", resp.SourcePath, map[string]string{"kind": resp.Kind})

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handlePreviewDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.DocumentPreviewRequest
	if err := decodeJSONBody(w, req, largeJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}

	resp, err := r.service.PreviewDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleDocumentStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.DocumentStatusRequest
	if err := decodeJSONBody(w, req, mediumJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}

	resp, err := r.service.UpdateDocumentStatus(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "document.status", "success", resp.SourcePath, map[string]string{"status": resp.Status})

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleDeleteDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.DocumentDeleteRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}

	resp, err := r.service.DeleteDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "document.delete", "success", resp.SourcePath, map[string]string{"trash_path": resp.TrashPath})

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMedia(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	items, err := r.service.ListMedia(req.Context(), strings.TrimSpace(req.URL.Query().Get("q")))
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (r *Router) handleMediaDetail(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	reference := strings.TrimSpace(req.URL.Query().Get("reference"))
	if reference == "" {
		writeJSONErrorMessage(w, http.StatusBadRequest, "missing required query parameter: reference")
		return
	}
	item, err := r.service.GetMediaDetail(req.Context(), reference)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (r *Router) handleMediaHistory(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	identifier := strings.TrimSpace(req.URL.Query().Get("reference"))
	if identifier == "" {
		identifier = strings.TrimSpace(req.URL.Query().Get("path"))
	}
	if identifier == "" {
		writeJSONErrorMessage(w, http.StatusBadRequest, "missing required query parameter: reference or path")
		return
	}
	resp, err := r.service.GetMediaHistory(req.Context(), identifier)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMediaTrash(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := r.service.ListMediaTrash(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMediaUpload(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, adminMultipartMaxBody)
	if err := req.ParseMultipartForm(adminMediaUploadLimit); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSONErrorMessage(w, http.StatusRequestEntityTooLarge, "media upload exceeds allowed size")
			return
		}
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	body, err := io.ReadAll(io.LimitReader(file, adminMediaUploadLimit+1))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := r.service.SaveMedia(
		req.Context(),
		req.FormValue("collection"),
		req.FormValue("dir"),
		header.Filename,
		header.Header.Get("Content-Type"),
		body,
	)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "media.upload", "success", resp.Reference, map[string]string{"created": boolString(resp.Created)})

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMediaReplace(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req.Body = http.MaxBytesReader(w, req.Body, adminMultipartMaxBody)
	if err := req.ParseMultipartForm(adminMediaUploadLimit); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSONErrorMessage(w, http.StatusRequestEntityTooLarge, "media upload exceeds allowed size")
			return
		}
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	reference := strings.TrimSpace(req.FormValue("reference"))
	if reference == "" {
		writeJSONErrorMessage(w, http.StatusBadRequest, "reference is required")
		return
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, adminMediaUploadLimit+1))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := r.service.ReplaceMedia(req.Context(), reference, header.Header.Get("Content-Type"), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "media.replace", "success", resp.Reference, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMediaDelete(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.MediaDeleteRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	if err := r.service.DeleteMedia(req.Context(), body.Reference); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "media.delete", "success", body.Reference, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleMediaRestore(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.MediaLifecycleRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.RestoreMedia(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "media.restore", "success", firstNonEmpty(resp.RestoredPath, resp.Path), nil)
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMediaPurge(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.MediaLifecycleRequest
	if err := decodeJSONBody(w, req, smallJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.PurgeMedia(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "media.purge", "success", resp.Path, nil)
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMediaMetadata(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.MediaMetadataSaveRequest
	if err := decodeJSONBody(w, req, mediumJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	body.Actor = actorLabel(req)
	resp, err := r.service.SaveMediaMetadata(req.Context(), body.Reference, body.Metadata, body.VersionComment, body.Actor)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "media.metadata", "success", body.Reference, nil)
	writeJSON(w, http.StatusOK, resp)
}

func actorLabel(req *http.Request) string {
	identity, ok := adminauth.IdentityFromContext(req.Context())
	if !ok {
		return ""
	}
	if name := strings.TrimSpace(identity.Name); name != "" {
		return name
	}
	return strings.TrimSpace(identity.Username)
}

func actorUsername(req *http.Request) string {
	identity, ok := adminauth.IdentityFromContext(req.Context())
	if !ok {
		return ""
	}
	return strings.TrimSpace(identity.Username)
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSONErrorMessage(w, status, err.Error())
}

func writeJSONErrorMessage(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{
		"error": msg,
	})
}
