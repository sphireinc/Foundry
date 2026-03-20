package types

import "time"

type AuditEntry struct {
	Timestamp  time.Time         `json:"timestamp"`
	Action     string            `json:"action"`
	Actor      string            `json:"actor,omitempty"`
	ActorRole  string            `json:"actor_role,omitempty"`
	RemoteAddr string            `json:"remote_addr,omitempty"`
	Target     string            `json:"target,omitempty"`
	Outcome    string            `json:"outcome,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}
