package transport_test

import (
	"testing"

	"github.com/reddit/baseplate.go/transport"
)

func TestDTabs_HappyPath(t *testing.T) {
	dtab := "prod-service=>test-service;another-service=>different-test-service"
	expected := map[string]string{
		"prod-service":    "test-service",
		"another-service": "different-test-service",
	}
	parsed, err := transport.ParseDTabs(dtab)
	if err != nil {
		t.Error(err, "Could not parse DTabs")
	}
	if len(parsed) != 2 {
		t.Errorf("Expected to see two items in %v", parsed)
	}
	for expectedK, expectedV := range expected {
		if parsed[expectedK] != expectedV {
			t.Errorf("Expected %s to map to %s, but got %s in %v", expectedK, expectedV, parsed[expectedK], parsed)
		}
	}
}

func TestDTabs_WithWhatespaces(t *testing.T) {
	dtab := "; prod-service => test-service ; another-service =>  different-test-service  ;  "
	expected := map[string]string{
		"prod-service":    "test-service",
		"another-service": "different-test-service",
	}
	parsed, err := transport.ParseDTabs(dtab)
	if err != nil {
		t.Error(err, "Could not parse DTabs")
	}
	if len(parsed) != 2 {
		t.Errorf("Expected to see two items in %v", parsed)
	}
	for expectedK, expectedV := range expected {
		if parsed[expectedK] != expectedV {
			t.Errorf("Expected %s to map to %s, but got %s in %v", expectedK, expectedV, parsed[expectedK], parsed)
		}
	}
}

func TestDTabs_Malformed(t *testing.T) {
	dtab := "prod-service=>test-service=>something-is-wrong-here;another-service=>different-test-service"
	parsed, err := transport.ParseDTabs(dtab)
	if err == nil {
		t.Error(err, "Expected to see an error")
	}
	if len(parsed) != 0 {
		t.Errorf("Expected to see no items in %v", parsed)
	}
}
