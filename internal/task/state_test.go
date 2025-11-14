package task

import "testing"

func TestValidStateTransition(t *testing.T) {
	tests := []struct {
		name     string
		src      State
		dst      State
		expected bool
	}{
		// Valid transitions from Pending
		{
			name:     "Pending to Scheduled",
			src:      Pending,
			dst:      Scheduled,
			expected: true,
		},
		{
			name:     "Pending to Running - invalid",
			src:      Pending,
			dst:      Running,
			expected: false,
		},
		{
			name:     "Pending to Completed - invalid",
			src:      Pending,
			dst:      Completed,
			expected: false,
		},
		{
			name:     "Pending to Failed - invalid",
			src:      Pending,
			dst:      Failed,
			expected: false,
		},
		// Valid transitions from Scheduled
		{
			name:     "Scheduled to Scheduled - rescheduling",
			src:      Scheduled,
			dst:      Scheduled,
			expected: true,
		},
		{
			name:     "Scheduled to Running",
			src:      Scheduled,
			dst:      Running,
			expected: true,
		},
		{
			name:     "Scheduled to Failed",
			src:      Scheduled,
			dst:      Failed,
			expected: true,
		},
		{
			name:     "Scheduled to Completed - invalid",
			src:      Scheduled,
			dst:      Completed,
			expected: false,
		},
		{
			name:     "Scheduled to Pending - invalid",
			src:      Scheduled,
			dst:      Pending,
			expected: false,
		},
		// Valid transitions from Running
		{
			name:     "Running to Running - continue",
			src:      Running,
			dst:      Running,
			expected: true,
		},
		{
			name:     "Running to Completed",
			src:      Running,
			dst:      Completed,
			expected: true,
		},
		{
			name:     "Running to Failed",
			src:      Running,
			dst:      Failed,
			expected: true,
		},
		{
			name:     "Running to Pending - invalid",
			src:      Running,
			dst:      Pending,
			expected: false,
		},
		{
			name:     "Running to Scheduled - invalid",
			src:      Running,
			dst:      Scheduled,
			expected: false,
		},
		// Terminal state: Completed
		{
			name:     "Completed to Pending - invalid",
			src:      Completed,
			dst:      Pending,
			expected: false,
		},
		{
			name:     "Completed to Scheduled - invalid",
			src:      Completed,
			dst:      Scheduled,
			expected: false,
		},
		{
			name:     "Completed to Running - invalid",
			src:      Completed,
			dst:      Running,
			expected: false,
		},
		{
			name:     "Completed to Failed - invalid",
			src:      Completed,
			dst:      Failed,
			expected: false,
		},
		{
			name:     "Completed to Completed - invalid",
			src:      Completed,
			dst:      Completed,
			expected: false,
		},
		// Terminal state: Failed
		{
			name:     "Failed to Pending - invalid",
			src:      Failed,
			dst:      Pending,
			expected: false,
		},
		{
			name:     "Failed to Scheduled - invalid",
			src:      Failed,
			dst:      Scheduled,
			expected: false,
		},
		{
			name:     "Failed to Running - invalid",
			src:      Failed,
			dst:      Running,
			expected: false,
		},
		{
			name:     "Failed to Completed - invalid",
			src:      Failed,
			dst:      Completed,
			expected: false,
		},
		{
			name:     "Failed to Failed - invalid",
			src:      Failed,
			dst:      Failed,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidStateTransition(tt.src, tt.dst)
			if result != tt.expected {
				t.Errorf("ValidStateTransition(%v, %v) = %v, expected %v",
					tt.src, tt.dst, result, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		states   []State
		state    State
		expected bool
	}{
		{
			name:     "Empty slice",
			states:   []State{},
			state:    Pending,
			expected: false,
		},
		{
			name:     "State found at beginning",
			states:   []State{Pending, Scheduled, Running},
			state:    Pending,
			expected: true,
		},
		{
			name:     "State found in middle",
			states:   []State{Pending, Scheduled, Running},
			state:    Scheduled,
			expected: true,
		},
		{
			name:     "State found at end",
			states:   []State{Pending, Scheduled, Running},
			state:    Running,
			expected: true,
		},
		{
			name:     "State not found",
			states:   []State{Pending, Scheduled, Running},
			state:    Completed,
			expected: false,
		},
		{
			name:     "Single element - found",
			states:   []State{Failed},
			state:    Failed,
			expected: true,
		},
		{
			name:     "Single element - not found",
			states:   []State{Failed},
			state:    Pending,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.states, tt.state)
			if result != tt.expected {
				t.Errorf("contains(%v, %v) = %v, expected %v",
					tt.states, tt.state, result, tt.expected)
			}
		})
	}
}

func TestStateTransitionMap(t *testing.T) {
	t.Run("All states defined in transition map", func(t *testing.T) {
		allStates := []State{Pending, Scheduled, Running, Completed, Failed}
		for _, state := range allStates {
			if _, exists := stateTransitionMap[state]; !exists {
				t.Errorf("State %v is not defined in stateTransitionMap", state)
			}
		}
	})

	t.Run("Terminal states have no transitions", func(t *testing.T) {
		terminalStates := []State{Completed, Failed}
		for _, state := range terminalStates {
			transitions := stateTransitionMap[state]
			if len(transitions) != 0 {
				t.Errorf("Terminal state %v should have no transitions, but has %d",
					state, len(transitions))
			}
		}
	})

	t.Run("Non-terminal states have at least one transition", func(t *testing.T) {
		nonTerminalStates := []State{Pending, Scheduled, Running}
		for _, state := range nonTerminalStates {
			transitions := stateTransitionMap[state]
			if len(transitions) == 0 {
				t.Errorf("Non-terminal state %v should have at least one transition",
					state)
			}
		}
	})
}

func TestStateTransitionScenarios(t *testing.T) {
	t.Run("Happy path: Pending -> Scheduled -> Running -> Completed", func(t *testing.T) {
		transitions := [][2]State{
			{Pending, Scheduled},
			{Scheduled, Running},
			{Running, Completed},
		}

		for _, trans := range transitions {
			if !ValidStateTransition(trans[0], trans[1]) {
				t.Errorf("Expected valid transition from %v to %v", trans[0], trans[1])
			}
		}
	})

	t.Run("Failure path: Scheduled -> Failed", func(t *testing.T) {
		if !ValidStateTransition(Scheduled, Failed) {
			t.Error("Expected valid transition from Scheduled to Failed")
		}
	})

	t.Run("Failure path: Running -> Failed", func(t *testing.T) {
		if !ValidStateTransition(Running, Failed) {
			t.Error("Expected valid transition from Running to Failed")
		}
	})

	t.Run("Rescheduling: Scheduled -> Scheduled", func(t *testing.T) {
		if !ValidStateTransition(Scheduled, Scheduled) {
			t.Error("Expected valid transition from Scheduled to Scheduled")
		}
	})

	t.Run("Long running task: Running -> Running", func(t *testing.T) {
		if !ValidStateTransition(Running, Running) {
			t.Error("Expected valid transition from Running to Running")
		}
	})
}
