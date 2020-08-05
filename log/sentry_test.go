package log

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type pair struct {
	key, value string
}

func TestExtractKeyValuePairs(t *testing.T) {
	for _, c := range []struct {
		label    string
		kv       []interface{}
		expected []pair
		dangling bool
	}{
		{
			label: "empty",
		},
		{
			label: "normal",
			kv: []interface{}{
				"key1", "value1",
				"key2", 2,
				"key3", 3.14,
			},
			expected: []pair{
				{
					key:   "key1",
					value: "value1",
				},
				{
					key:   "key2",
					value: "2",
				},
				{
					key:   "key3",
					value: "3.14",
				},
			},
		},
		{
			label: "ignore-zap-field",
			kv: []interface{}{
				"key1", "value1",
				"key2", "value2",
				zap.Field{},     // will be ignored
				zapcore.Field{}, // will be ignored
			},
			expected: []pair{
				{
					key:   "key1",
					value: "value1",
				},
				{
					key:   "key2",
					value: "value2",
				},
			},
		},
		{
			label: "dangling",
			kv: []interface{}{
				"key1", "value1",
				"dangling",
			},
			expected: []pair{
				{
					key:   "key1",
					value: "value1",
				},
			},
			dangling: true,
		},
		{
			label: "dangling-with-field",
			kv: []interface{}{
				"key1", "value1",
				zap.Field{},
				"dangling",
			},
			expected: []pair{
				{
					key:   "key1",
					value: "value1",
				},
			},
			dangling: true,
		},
	} {
		t.Run(
			c.label,
			func(t *testing.T) {
				var called int
				f := func(key, value string) {
					t.Helper()
					defer func() {
						called++
					}()
					if called >= len(c.expected) {
						t.Errorf("Extra call with (%q, %q)", key, value)
						return
					}
					if c.expected[called].key != key || c.expected[called].value != value {
						t.Errorf(
							"Expected %#v on %dth call, got (%q, %q)",
							c.expected[called],
							called,
							key, value,
						)
					}
				}
				dangling := extractKeyValuePairs(c.kv, f)
				if dangling != c.dangling {
					t.Errorf("Expected dangling to return %v, got %v", c.dangling, dangling)
				}
				if called < len(c.expected) {
					t.Errorf("Expected %d calls, got %v", len(c.expected), called)
				}
			},
		)
	}
}
