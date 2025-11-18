package services

import "testing"

func TestNew(t *testing.T) {
	s := New()

	if s == nil {
		t.Fatal("New() returned nil")
	}

	t.Run("MemStats should not be nil", func(t *testing.T) {
		if s.MemStats == nil {
			t.Error("MemStats is nil")
		}
	})

	t.Run("DiskStats should not be nil", func(t *testing.T) {
		if s.DiskStats == nil {
			t.Error("DiskStats is nil")
		}
	})

	t.Run("CPUStats should not be nil", func(t *testing.T) {
		if s.CPUStats == nil {
			t.Error("CPUStats is nil")
		}
	})

	t.Run("LoadStats should not be nil", func(t *testing.T) {
		if s.LoadStats == nil {
			t.Error("LoadStats is nil")
		}
	})

	t.Run("TaskCount should be initialized to 0", func(t *testing.T) {
		if s.TaskCount != 0 {
			t.Errorf("TaskCount = %d, want = 0", s.TaskCount)
		}
	})
}
