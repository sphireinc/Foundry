package content

import (
	"strings"
	"time"
)

type Workflow struct {
	Status             string
	Archived           bool
	InReview           bool
	ScheduledPublish   *time.Time
	ScheduledUnpublish *time.Time
	EditorialNote      string
}

func WorkflowFromFrontMatter(fm *FrontMatter, now time.Time) Workflow {
	workflow := Workflow{Status: "published"}
	if fm == nil {
		return workflow
	}
	if fm.Draft {
		workflow.Status = "draft"
	}
	if fm.Params != nil {
		if value, ok := fm.Params["workflow"].(string); ok && normalizeWorkflowStatus(value) != "" {
			workflow.Status = normalizeWorkflowStatus(value)
		}
		if archived, ok := fm.Params["archived"].(bool); ok && archived {
			workflow.Status = "archived"
			workflow.Archived = true
		}
		if note, ok := fm.Params["editorial_note"].(string); ok {
			workflow.EditorialNote = strings.TrimSpace(note)
		}
		if ts := timeFromParam(fm.Params["scheduled_publish_at"]); ts != nil {
			workflow.ScheduledPublish = ts
			if workflow.Status == "published" && now.Before(*ts) {
				workflow.Status = "scheduled"
			}
		}
		if ts := timeFromParam(fm.Params["scheduled_unpublish_at"]); ts != nil {
			workflow.ScheduledUnpublish = ts
			if now.After(*ts) {
				workflow.Status = "draft"
			}
		}
	}
	workflow.InReview = workflow.Status == "in_review"
	if workflow.Status == "archived" {
		workflow.Archived = true
	}
	return workflow
}

func normalizeWorkflowStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draft", "in_review", "scheduled", "published", "archived":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func timeFromParam(value any) *time.Time {
	switch typed := value.(type) {
	case time.Time:
		t := typed
		return &t
	case *time.Time:
		return typed
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return nil
		}
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
			if parsed, err := time.Parse(layout, typed); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

func ApplyWorkflowToFrontMatter(fm *FrontMatter, status string, publishAt, unpublishAt *time.Time, note string) {
	if fm == nil {
		return
	}
	if fm.Params == nil {
		fm.Params = make(map[string]any)
	}
	status = normalizeWorkflowStatus(status)
	if status == "" {
		status = "draft"
	}
	fm.Params["workflow"] = status
	switch status {
	case "draft":
		fm.Draft = true
		delete(fm.Params, "archived")
	case "in_review":
		fm.Draft = true
		delete(fm.Params, "archived")
	case "scheduled":
		fm.Draft = true
		delete(fm.Params, "archived")
	case "published":
		fm.Draft = false
		delete(fm.Params, "archived")
	case "archived":
		fm.Draft = true
		fm.Params["archived"] = true
	}
	if publishAt != nil {
		fm.Params["scheduled_publish_at"] = publishAt.UTC().Format(time.RFC3339)
	} else {
		delete(fm.Params, "scheduled_publish_at")
	}
	if unpublishAt != nil {
		fm.Params["scheduled_unpublish_at"] = unpublishAt.UTC().Format(time.RFC3339)
	} else {
		delete(fm.Params, "scheduled_unpublish_at")
	}
	note = strings.TrimSpace(note)
	if note != "" {
		fm.Params["editorial_note"] = note
	} else {
		delete(fm.Params, "editorial_note")
	}
}
