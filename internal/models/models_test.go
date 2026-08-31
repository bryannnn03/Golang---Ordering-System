package models

import (
	"testing"
)

func TestOrderStatusValidation(t *testing.T) {
	tests := []struct {
		status OrderStatus
		valid  bool
	}{
		{StatusPending, true},
		{StatusProcessing, true},
		{StatusCompleted, true},
		{StatusCancelled, true},
		{OrderStatus("INVALID_STATUS"), false},
		{OrderStatus(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("OrderStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}
