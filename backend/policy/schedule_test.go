package policy

import (
	"goaway/backend/database"
	"testing"
	"time"
)

// Reference instant: Wednesday 2026-07-22, used with an explicit clock below.
func at(hhmm string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", "2026-07-22 "+hhmm)
	if err != nil {
		panic(err)
	}
	return t
}

func TestIsScheduleActive(t *testing.T) {
	s := &Service{}

	tests := []struct {
		name  string
		sched *database.Schedule
		now   time.Time
		want  bool
	}{
		{
			name:  "nil schedule is always active",
			sched: nil,
			now:   at("03:00"),
			want:  true,
		},
		{
			name:  "inside daytime window",
			sched: &database.Schedule{Days: "mon,tue,wed", StartTime: "08:00", EndTime: "17:00"},
			now:   at("12:00"),
			want:  true,
		},
		{
			name:  "before daytime window",
			sched: &database.Schedule{Days: "mon,tue,wed", StartTime: "08:00", EndTime: "17:00"},
			now:   at("07:59"),
			want:  false,
		},
		{
			name:  "after daytime window",
			sched: &database.Schedule{Days: "mon,tue,wed", StartTime: "08:00", EndTime: "17:00"},
			now:   at("17:01"),
			want:  false,
		},
		{
			name:  "wrong day of week",
			sched: &database.Schedule{Days: "sat,sun", StartTime: "08:00", EndTime: "17:00"},
			now:   at("12:00"), // a Wednesday
			want:  false,
		},
		{
			name:  "empty days means every day",
			sched: &database.Schedule{StartTime: "08:00", EndTime: "17:00"},
			now:   at("12:00"),
			want:  true,
		},
		// Overnight windows: the common "block social media 22:00-06:00" rule.
		{
			name:  "overnight window, late evening",
			sched: &database.Schedule{StartTime: "22:00", EndTime: "06:00"},
			now:   at("23:30"),
			want:  true,
		},
		{
			name:  "overnight window, early morning",
			sched: &database.Schedule{StartTime: "22:00", EndTime: "06:00"},
			now:   at("02:00"),
			want:  true,
		},
		{
			name:  "overnight window, midday is outside",
			sched: &database.Schedule{StartTime: "22:00", EndTime: "06:00"},
			now:   at("13:00"),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.isScheduleActive(tt.sched, tt.now); got != tt.want {
				t.Errorf("isScheduleActive() = %v, want %v", got, tt.want)
			}
		})
	}
}
