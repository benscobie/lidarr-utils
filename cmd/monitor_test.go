package cmd

import (
	"slices"
	"testing"
)

func TestLabelIDsForRunPositionalOverridesConfig(t *testing.T) {
	got, err := labelIDsForRun(
		[]string{"11111111-1111-1111-1111-111111111111"},
		[]string{"22222222-2222-2222-2222-222222222222"},
	)
	if err != nil ||
		!slices.Equal(got, []string{"11111111-1111-1111-1111-111111111111"}) {
		t.Fatalf("unexpected IDs: %v, %v", got, err)
	}
}

func TestLabelIDsForRunRequiresAtLeastOneID(t *testing.T) {
	if _, err := labelIDsForRun(nil, nil); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestLabelIDsForRunRejectsMalformedPositionalID(t *testing.T) {
	if _, err := labelIDsForRun([]string{"not-a-uuid"}, nil); err == nil {
		t.Fatal("expected UUID validation error")
	}
}
