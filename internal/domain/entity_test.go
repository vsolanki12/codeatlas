package domain

import (
	"testing"
)

func TestEntityKindString(t *testing.T) {
	tests := []struct {
		kind     EntityKind
		expected string
	}{
		{KindOperator, "operator"},
		{KindController, "controller"},
		{KindCRD, "crd"},
		{KindFunction, "function"},
		{KindPackage, "package"},
		{KindTest, "test"},
		{KindDocument, "document"},
		{KindResource, "resource"},
		{KindUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.expected {
				t.Errorf("EntityKind.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}
