package httpadmin

import (
	"net/http"

	admintypes "github.com/sphireinc/foundry/internal/admin/types"
)

func (r *Router) handleRedirects(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := r.service.ListRedirects(req.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleSaveRedirects(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body admintypes.RedirectSaveRequest
	if err := decodeJSONBody(w, req, configJSONBodyLimit, &body); err != nil {
		if !writeRequestBodyError(w, err) {
			writeJSONError(w, http.StatusBadRequest, err)
		}
		return
	}
	resp, err := r.service.SaveRedirects(req.Context(), body.Redirects)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	r.logAuditRequest(req, "redirects.save", "success", resp.Path, nil)
	writeJSON(w, http.StatusOK, resp)
}
