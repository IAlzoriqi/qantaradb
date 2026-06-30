package loader

import "testing"

func TestCompletedStateResumeActionRequiresRowCountParity(t *testing.T) {
	tests := []struct {
		name       string
		sourceRows int64
		targetRows int64
		want       completedStateAction
	}{
		{
			name:       "verified parity can skip",
			sourceRows: 42,
			targetRows: 42,
			want:       completedStateSkip,
		},
		{
			name:       "clean target recopy is allowed",
			sourceRows: 42,
			targetRows: 0,
			want:       completedStateRecopyClean,
		},
		{
			name:       "partial target cannot skip",
			sourceRows: 42,
			targetRows: 12,
			want:       completedStateFail,
		},
		{
			name:       "unexpected extra target rows cannot skip",
			sourceRows: 42,
			targetRows: 50,
			want:       completedStateFail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := completedStateResumeAction(tt.sourceRows, tt.targetRows); got != tt.want {
				t.Fatalf("completedStateResumeAction(%d, %d) = %s, want %s", tt.sourceRows, tt.targetRows, got, tt.want)
			}
		})
	}
}
