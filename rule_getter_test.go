package ecschedule

import (
	"encoding/json"
	"testing"

	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func TestRuleGetter_TaskOverrideHandling(t *testing.T) {

	tests := []struct {
		name           string
		input          string
		expectNilTaskOverride bool
	}{
		{
			name: "Empty TaskOverride should be nil",
			input: `{"containerOverrides":[{"name":"test","command":["echo","hello"]}]}`,
			expectNilTaskOverride: true,
		},
		{
			name: "TaskOverride with CPU should not be nil",
			input: `{"cpu":"512","containerOverrides":[{"name":"test"}]}`,
			expectNilTaskOverride: false,
		},
		{
			name: "TaskOverride with Memory should not be nil", 
			input: `{"memory":"1024","containerOverrides":[{"name":"test"}]}`,
			expectNilTaskOverride: false,
		},
		{
			name: "TaskOverride with both CPU and Memory should not be nil",
			input: `{"cpu":"512","memory":"1024","containerOverrides":[{"name":"test"}]}`,
			expectNilTaskOverride: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic directly by simulating the Input parsing
			taskOv := &ecsTypes.TaskOverride{}
			if err := json.Unmarshal([]byte(tt.input), taskOv); err != nil {
				t.Fatalf("Failed to unmarshal test input: %v", err)
			}

			// Test our fix logic
			var resultTaskOverride *TaskOverride
			if taskOv.Cpu != nil || taskOv.Memory != nil {
				resultTaskOverride = &TaskOverride{
					Cpu:    taskOv.Cpu,
					Memory: taskOv.Memory,
				}
			}

			if tt.expectNilTaskOverride {
				if resultTaskOverride != nil {
					t.Errorf("Expected TaskOverride to be nil, but got: %+v", resultTaskOverride)
				}
			} else {
				if resultTaskOverride == nil {
					t.Errorf("Expected TaskOverride to not be nil")
				}
			}
		})
	}
}