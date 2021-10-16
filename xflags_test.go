package xflags

import (
	"testing"
	"time"
)

func assertBool(t *testing.T, expect, actual bool) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected bool: %v, got: %v", expect, actual)
	return false
}

func assertDuration(t *testing.T, expect, actual time.Duration) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected duration: %v, got: %v", expect, actual)
	return false
}

func assertFloat64(t *testing.T, expect, actual float64) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected float64: %v, got: %v", expect, actual)
	return false
}

func assertInt64(t *testing.T, expect, actual int64) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected int64: %v, got: %v", expect, actual)
	return false
}

func assertUint64(t *testing.T, expect, actual uint64) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected uint64: 0x%0X, got: 0x%0X", expect, actual)
	return false
}

func assertString(t *testing.T, expect, actual string) bool {
	if expect == actual {
		return true
	}
	t.Errorf("expected string: \"%s\", got: \"%s\"", expect, actual)
	return false
}

func assertStringSlice(t *testing.T, expect, actual []string) bool {
	if len(expect) != len(actual) {
		t.Errorf("expected string slice: %q, got: %q", expect, actual)
		return false
	}
	for i := 0; i < len(expect); i++ {
		if expect[i] != actual[i] {
			t.Errorf("expected string slice: %q, got: %q", expect, actual)
			return false
		}
	}
	return true
}
