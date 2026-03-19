package httpadmin

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

const adminMediaUploadLimit = 256 << 20

func registerDocumentRoutes(r *Router) []routeDef {
	return []routeDef{
		{
			pattern: "/__admin/api/documents",
			handler: http.HandlerFunc(r.handleDocuments),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/document",
			handler: http.HandlerFunc(r.handleDocument),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/documents/create",
			handler: http.HandlerFunc(r.handleCreateDocument),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/documents/save",
			handler: http.HandlerFunc(r.handleSaveDocument),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/documents/status",
			handler: http.HandlerFunc(r.handleDocumentStatus),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/documents/delete",
			handler: http.HandlerFunc(r.handleDeleteDocument),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/documents/preview",
			handler: http.HandlerFunc(r.handlePreviewDocument),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/media",
			handler: http.HandlerFunc(r.handleMedia),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/media/detail",
			handler: http.HandlerFunc(r.handleMediaDetail),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/media/upload",
			handler: http.HandlerFunc(r.handleMediaUpload),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/media/metadata",
			handler: http.HandlerFunc(r.handleMediaMetadata),
			role:    "editor",
		},
		{
			pattern: "/__admin/api/media/delete",
			handler: http.HandlerFunc(r.handleMediaDelete),
			role:    "editor",
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
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := r.service.SaveDocument(req.Context(), body)
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
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := r.service.CreateDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handlePreviewDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.DocumentPreviewRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
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
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := r.service.UpdateDocumentStatus(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleDeleteDocument(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body admintypes.DocumentDeleteRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	resp, err := r.service.DeleteDocument(req.Context(), body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMedia(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	items, err := r.service.ListMedia(req.Context())
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

func (r *Router) handleMediaUpload(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := req.ParseMultipartForm(adminMediaUploadLimit); err != nil {
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

	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleMediaDelete(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.MediaDeleteRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	if err := r.service.DeleteMedia(req.Context(), body.Reference); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) handleMediaMetadata(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.MediaMetadataSaveRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	resp, err := r.service.SaveMediaMetadata(req.Context(), body.Reference, body.Metadata)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
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
