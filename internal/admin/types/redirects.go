package types

type RedirectRule struct {
	From          string `json:"from"`
	To            string `json:"to"`
	Status        int    `json:"status"`
	Enabled       bool   `json:"enabled"`
	PreserveQuery bool   `json:"preserve_query,omitempty"`
	Note          string `json:"note,omitempty"`
}

type RedirectListResponse struct {
	Path      string         `json:"path"`
	Redirects []RedirectRule `json:"redirects"`
}

type RedirectSaveRequest struct {
	Redirects []RedirectRule `json:"redirects"`
}
