package domain

import (
	"testing"
	"time"
)

func TestEventListItem_Fields(t *testing.T) {
	now := time.Now()
	item := EventListItem{
		ID:            "abc-123",
		Title:         "Test Event",
		Location:      "Camp Grounds",
		StartTime:     now,
		EndTime:       now.Add(2 * time.Hour),
		Type:          "campout",
		AttendeeCount: 5,
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"ID", item.ID, "abc-123"},
		{"Title", item.Title, "Test Event"},
		{"Location", item.Location, "Camp Grounds"},
		{"StartTime", item.StartTime, now},
		{"EndTime", item.EndTime, now.Add(2 * time.Hour)},
		{"Type", item.Type, "campout"},
		{"AttendeeCount", item.AttendeeCount, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("EventListItem.%s = %v, want %v", tt.name, tt.got, tt.want)
			}
		})
	}
}
