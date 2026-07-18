package parser

import (
	"testing"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

func TestTestParser_Parse(t *testing.T) {
	tp := NewTestParser()
	targetFile := domain.File{
		RelativePath: "testdata/sample_test.go",
	}

	entities, err := tp.Parse(targetFile)
	if err != nil {
		t.Fatalf("TestParser.Parse failed unexpectedly: %v", err)
	}

	// We expect exactly 2 entities (TestLogin and TestLogout). HelperUtility must be ignored.
	if len(entities) != 2 {
		t.Fatalf("Expected exactly 2 test entities, got %d", len(entities))
	}

	// Build a fast lookup map to evaluate metadata targets
	byName := make(map[string]domain.Entity)
	for _, ent := range entities {
		byName[ent.Name] = ent
	}

	// 1. Assert specific entity details for TestLogin
	t.Run("Verify TestLogin Metadata", func(t *testing.T) {
		loginTest, exists := byName["TestLogin"]
		if !exists {
			t.Fatal("Missing expected 'TestLogin' entity")
		}

		expectedID := "test:sample.TestLogin"
		if loginTest.ID != expectedID {
			t.Errorf("ID mismatch. Got %q, want %q", loginTest.ID, expectedID)
		}

		if loginTest.Kind != domain.KindTest {
			t.Errorf("Expected KindTest, got structural kind value: %v", loginTest.Kind)
		}

		expectedDoc := "TestLogin verifies the authentication middleware pipeline."
		if loginTest.Description != expectedDoc {
			t.Errorf("Description documentation mismatch.\nExpected: %q\nGot: %q", expectedDoc, loginTest.Description)
		}
	})

	// 2. Assert specific entity details for TestLogout
	t.Run("Verify TestLogout Presence", func(t *testing.T) {
		_, exists := byName["TestLogout"]
		if !exists {
			t.Error("Missing expected 'TestLogout' entity")
		}
	})
}
