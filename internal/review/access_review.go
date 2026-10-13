package review

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/health-access-review/internal/infrai"
)

type AppointmentChange struct {
	PatientRef  string `json:"patient_ref"`
	Clinic      string `json:"clinic"`
	ScheduledAt string `json:"scheduled_at"`
	Status      string `json:"status"`
}

type NotificationDecision struct {
	PatientRef string `json:"patient_ref"`
	Send       bool   `json:"send"`
	Message    string `json:"message,omitempty"`
	Reason     string `json:"reason"`
}

type KeyReach struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Read       []string `json:"read"`
	Change     []string `json:"change"`
	Other      []string `json:"other"`
	BroadScope bool     `json:"broad_scope"`
}

type Report struct {
	Date          string                 `json:"date"`
	Keys          []KeyReach             `json:"keys"`
	Notifications []NotificationDecision `json:"notifications"`
}

func Build(keys []infrai.Key, changes []AppointmentChange, now time.Time) Report {
	report := Report{Date: now.UTC().Format("2006-01-02")}
	for _, key := range keys {
		reach := KeyReach{ID: key.ID, Name: key.Name}
		for _, scope := range key.Scopes {
			switch {
			case scope == "*":
				reach.BroadScope = true
				reach.Other = append(reach.Other, scope)
			case strings.HasSuffix(scope, ".read"), strings.HasSuffix(scope, ".list"), strings.HasSuffix(scope, ".get"):
				reach.Read = append(reach.Read, scope)
			case strings.HasSuffix(scope, ".create"), strings.HasSuffix(scope, ".update"), strings.HasSuffix(scope, ".send"), strings.HasSuffix(scope, ".write"):
				reach.Change = append(reach.Change, scope)
			default:
				reach.Other = append(reach.Other, scope)
			}
		}
		sort.Strings(reach.Read)
		sort.Strings(reach.Change)
		sort.Strings(reach.Other)
		report.Keys = append(report.Keys, reach)
	}
	for _, change := range changes {
		report.Notifications = append(report.Notifications, decideNotification(change))
	}
	return report
}

func decideNotification(change AppointmentChange) NotificationDecision {
	decision := NotificationDecision{PatientRef: change.PatientRef}
	switch change.Status {
	case "scheduled", "rescheduled", "cancelled":
		if change.PatientRef == "" || change.Clinic == "" || change.ScheduledAt == "" {
			decision.Reason = "missing operational appointment detail"
			return decision
		}
		decision.Send = true
		decision.Reason = "operational appointment update"
		decision.Message = fmt.Sprintf("Appointment %s at %s on %s. Reference: %s.", change.Status, change.Clinic, change.ScheduledAt, change.PatientRef)
		return decision
	default:
		decision.Reason = "status is not patient-facing"
		return decision
	}
}

func Markdown(report Report) string {
	var out strings.Builder
	fmt.Fprintf(&out, "# Health access review - %s\n\n", report.Date)
	fmt.Fprintf(&out, "## API key reach\n\n")
	for _, key := range report.Keys {
		fmt.Fprintf(&out, "### %s (`%s`)\n\n", key.Name, key.ID)
		fmt.Fprintf(&out, "- Read: %s\n", displayScopes(key.Read))
		fmt.Fprintf(&out, "- Change: %s\n", displayScopes(key.Change))
		fmt.Fprintf(&out, "- Other: %s\n", displayScopes(key.Other))
		fmt.Fprintf(&out, "- Broad scope: %t\n\n", key.BroadScope)
	}
	fmt.Fprintf(&out, "## Appointment notifications\n\n")
	for _, item := range report.Notifications {
		if item.Send {
			fmt.Fprintf(&out, "- `%s`: send - %s\n", item.PatientRef, item.Message)
		} else {
			fmt.Fprintf(&out, "- `%s`: hold - %s\n", item.PatientRef, item.Reason)
		}
	}
	return out.String()
}

func displayScopes(scopes []string) string {
	if len(scopes) == 0 {
		return "none"
	}
	return strings.Join(scopes, ", ")
}
