package claims

import (
	"testing"

	"github.com/hemanth-shankar/expense-agent/internal/duplicate"
)

func TestDuplicateRequiresManualReview(t *testing.T) {
	tests := []struct {
		name           string
		matchType      string
		isDuplicate    bool
		strongIdentity bool
		want           bool
	}{
		{
			name:           "strong same employee metadata match",
			matchType:      duplicate.MatchMetadata,
			isDuplicate:    true,
			strongIdentity: true,
			want:           true,
		},
		{
			name:           "ordinary same employee metadata similarity",
			matchType:      duplicate.MatchMetadata,
			isDuplicate:    true,
			strongIdentity: false,
			want:           false,
		},
		{
			name:           "cross employee metadata is handled separately",
			matchType:      duplicate.MatchCrossEmployeeMetadata,
			isDuplicate:    false,
			strongIdentity: true,
			want:           false,
		},
		{
			name:           "exact hash does not use clarification path",
			matchType:      duplicate.MatchExact,
			isDuplicate:    true,
			strongIdentity: true,
			want:           false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := duplicateRequiresManualReview(
				test.matchType,
				test.isDuplicate,
				test.strongIdentity,
			)

			if got != test.want {
				t.Fatalf(
					"duplicateRequiresManualReview() = %v, want %v",
					got,
					test.want,
				)
			}
		})
	}
}
