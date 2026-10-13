package review

import (
	"strings"
	"testing"
	"time"

	"example.com/health-access-review/internal/infrai"
)

func TestBuildNotificationDecision(t *testing.T) {
	tests := []struct {
		name   string
		change AppointmentChange
		send   bool
		want   string
	}{
		{
			name: "reschedule contains operational detail",
			change: AppointmentChange{
				PatientRef: "P-1042", Clinic: "North Clinic", ScheduledAt: "2026-10-03 09:30", Status: "rescheduled",
			},
			send: true,
			want: "Appointment rescheduled at North Clinic on 2026-10-03 09:30. Reference: P-1042.",
		},
		{
			name:   "internal status stays internal",
			change: AppointmentChange{PatientRef: "P-1043", Clinic: "North Clinic", ScheduledAt: "2026-10-03 10:00", Status: "chart-reviewed"},
			send:   false,
			want:   "status is not patient-facing",
		},
		{
			name:   "incomplete update is held",
			change: AppointmentChange{PatientRef: "P-1044", Status: "cancelled"},
			send:   false,
			want:   "missing operational appointment detail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := Build(nil, []AppointmentChange{tt.change}, time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC))
			got := report.Notifications[0]
			if got.Send != tt.send {
				t.Fatalf("Send = %t, want %t", got.Send, tt.send)
			}
			text := got.Reason
			if got.Send {
				text = got.Message
			}
			if text != tt.want {
				t.Fatalf("decision text = %q, want %q", text, tt.want)
			}
		})
	}
}

func TestMarkdownGroupsReachAndDatesDocument(t *testing.T) {
	keys := []infrai.Key{{ID: "key_7", Name: "scheduler", Scopes: []string{"appointments.read", "appointments.update", "*"}}}
	report := Build(keys, nil, time.Date(2026, 9, 29, 23, 10, 0, 0, time.FixedZone("clinic", 8*60*60)))
	markdown := Markdown(report)
	for _, want := range []string{"2026-09-29", "appointments.read", "appointments.update", "Broad scope: true"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("document does not contain %q", want)
		}
	}
}
