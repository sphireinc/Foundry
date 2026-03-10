package httpadmin

import (
	"encoding/json"
	"net/http"
	"strings"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

func registerDocumentRoutes(r *Router) []routeDef {
	return []routeDef{
		{
			pattern: "/__admin/api/documents",
			handler: http.HandlerFunc(r.handleDocuments),
		},
		{
			pattern: "/__admin/api/document",
			handler: http.HandlerFunc(r.handleDocument),
		},
		{
			pattern: "/__admin/api/documents/save",
			handler: http.HandlerFunc(r.handleSaveDocument),
		},
		{
			pattern: "/__admin/api/documents/preview",
			handler: http.HandlerFunc(r.handlePreviewDocument),
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
