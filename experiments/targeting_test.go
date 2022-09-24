package experiments

import (
	"errors"
	"testing"
)

var targetingConfig = []byte(`{
	"ALL":[
		{"ANY":[
			{"EQ":{"field":"is_mod", "value":true}},
			{"EQ":{"field":"user_id", "values":["t2_1","t2_2","t2_3","t2_4"]}}
		]},
		{"NOT":{
			"EQ":{"field":"is_pita", "value":true}}},
		{"EQ":{"field":"is_logged_in", "values":[true, false]}},
		{"NOT":{
			"EQ":{"field":"subreddit_id", "values":["t5_1","t5_2"]}}},
		{"ALL":[
			{"EQ":{"field":"random_numeric","values":[1,2,3,4,5]}},
			{"EQ":{"field":"random_numeric","value":5}}
		]}
	]
}`)

func TestNominal(t *testing.T) {
	t.Parallel()
	targeting, err := NewTargeting(targetingConfig)
	if err != nil {
		t.Fatal(err)
	}

	inputs := make(map[string]interface{})
	inputs["user_id"] = "t2_1"
	inputs["is_mod"] = false
	inputs["is_pita"] = false
	inputs["random_numeric"] = 5

	result := targeting.Evaluate(inputs)
	if result {
		t.Errorf("expected targeting to evaluate false but evaluated %t", result)
	}

	inputs["is_logged_in"] = true

	result = targeting.Evaluate(inputs)
	if !result {
		t.Errorf("expected targeting to evaluate false but evaluated %t", result)
	}
}

func TestCreateTreeMultipleKeys(t *testing.T) {
	t.Parallel()
	targetingConfig := []byte(`{
	"ALL":[
		{"ANY":[
			{"EQ":{"field":"is_mod", "value":true}},
			{"EQ":{"field":"user_id", "values":["t2_1","t2_2","t2_3","t2_4"]}}
		]},
		{"NOT":{
			"EQ":{"field":"is_pita", "value":true}}},
		{"EQ":{"field":"is_logged_in", "values":[true, false]}},
		{"NOT":{
			"EQ":{"field":"subreddit_id", "values":["t5_1","t5_2"]}}},
		{"ALL":[
			{"EQ":{"field":"random_numeric","values":[1,2,3,4,5]}},
			{"EQ":{"field":"random_numeric","value":5}}
		]}
	],
	"ANY":[
		{"EQ":{"field":"is_mod", "value": true}},
		{"EQ":{"field":"user_id", "values": ["t2_1", "t2_2", "t2_3", "t2_4"]}}
	]
}`)

	var e TargetingNodeError
	_, err := NewTargeting(targetingConfig)
	if !errors.As(err, &e) {
		t.Errorf("expected TargetingNodeError, got: %v", err)
	}
}

func TestCreateTreeUnknownOperator(t *testing.T) {
	t.Parallel()
	targetingConfig := []byte(`{
		"UNKNOWN":[
			{"ANY":[
			{"EQ":{"field":"is_mod", "value":true}},
			{"EQ":{"field":"user_id", "values":["t2_1","t2_2","t2_3","t2_4"]}}
			]},
			{"NOT":{
				"EQ":{"field":"is_pita", "value":true}}},
				{"EQ":{"field":"is_logged_in", "values":[true, false]}},
				{"NOT":{
					"EQ":{"field":"subreddit_id", "values":["t5_1","t5_2"]}}},
					{"ALL":[
					{"EQ":{"field":"random_numeric","values":[1,2,3,4,5]}},
					{"EQ":{"field":"random_numeric","value":5}}
					]}
					]
				}`)
	var e UnknownTargetingOperatorError
	_, err := NewTargeting(targetingConfig)
	if !errors.As(err, &e) {
		t.Errorf("expected %T, got: %T", e, err)
	}
}

func TestEqualValueNode(t *testing.T) {
	tests := []struct {
		name         string
		targetConfig []byte
		expected     bool
	}{
		{
			name:         "bool",
			targetConfig: []byte(`{"EQ":{"field":"bool_field", "value":true}}`),
			expected:     true,
		},
		{
			name:         "number",
			targetConfig: []byte(`{"EQ":{"field":"num_field", "value":5}}`),
			expected:     true,
		},
		{
			name:         "string",
			targetConfig: []byte(`{"EQ":{"field":"str_field", "value":"string_value"}}`),
			expected:     true,
		},
		{
			name:         "nil",
			targetConfig: []byte(`{"EQ":{"field":"explicit_nil_field", "value":null}}`),
			expected:     true,
		},
		{
			name:         "field missing",
			targetConfig: []byte(`{"EQ":{"field":"implicit_nil_field", "value":null}}`),
			expected:     true,
		},
		{
			name:         "bool list",
			targetConfig: []byte(`{"EQ":{"field":"bool_field", "values":[true, false]}}`),
			expected:     true,
		},
		{
			name:         "number list",
			targetConfig: []byte(`{"EQ":{"field":"num_field", "values":[5, 6, 7, 8, 9]}}`),
			expected:     true,
		},
		{
			name:         "string list",
			targetConfig: []byte(`{"EQ":{"field":"str_field", "values":["string_value", "string_value_2", "string_value_3"]}}`),
			expected:     true,
		},
		{
			name:         "nil list",
			targetConfig: []byte(`{"EQ":{"field":"explicit_nil_field", "values":[null, true]}}`),
			expected:     true,
		},
		{
			name:         "field missing list",
			targetConfig: []byte(`{"EQ":{"field":"implicit_nil_field", "values":[null]}}`),
			expected:     true,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inputSet := inputSet()
			targetTree, err := NewTargeting(tt.targetConfig)
			if err != nil {
				t.Fatal(err)
			}
			result := targetTree.Evaluate(inputSet)
			if result != tt.expected {
				t.Errorf("expected to evaluate to %t, actual: %t", tt.expected, result)
			}
		})
	}
}

func TestEqualNodeBadInputs(t *testing.T) {
	tests := []struct {
		name         string
		targetConfig []byte
	}{
		{
			name:         "empty config",
			targetConfig: []byte(`{"EQ":{}}`),
		},
		{
			name:         "one argument",
			targetConfig: []byte(`{"EQ":{"field": "some_field"}}`),
		},
		{
			name:         "three arguments",
			targetConfig: []byte(`{"EQ":{"field": "some_field", "values": ["one", true], "value": "str_arg"}}`),
		},
		{
			name:         "no field",
			targetConfig: []byte(`{"EQ":{"fields": "some_field", "value": "str_arg"}}`),
		},
		{
			name:         "no value",
			targetConfig: []byte(`{"EQ":{"field": "some_field", "valu": "str_arg"}}`),
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewTargeting(tt.targetConfig)
			var expectedError TargetingNodeError
			if !errors.As(err, &expectedError) {
				t.Errorf("expected %T, got: %T", expectedError, err)
			}
		})
	}
}

func TestNotNode(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected bool
	}{
		{
			name:     "target hit",
			input:    map[string]interface{}{"str_field": "string_value"},
			expected: false,
		},
		{
			name:     "target miss",
			input:    map[string]interface{}{"str_field": "str_value"},
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			targetingConfig := []byte(`{"NOT":{"EQ":{"field": "str_field", "value": "string_value"}}}`)
			targetTree, err := NewTargeting(targetingConfig)
			if err != nil {
				t.Fatal(err)
			}
			result := targetTree.Evaluate(tt.input)
			if result != tt.expected {
				t.Errorf("expected result to be %t, actual: %t", tt.expected, result)
			}
		})
	}
}

func TestNotNodeBadInputs(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		t.Parallel()
		targetingConfig := []byte(`{"NOT":{}}`)
		_, err := NewTargeting(targetingConfig)
		var expectedError TargetingNodeError
		if !errors.As(err, &expectedError) {
			t.Errorf("expected error %T, actual %v (%T)", expectedError, err, err)
		}
	})
	t.Run("multiple arguments config", func(t *testing.T) {
		t.Parallel()
		targetingConfig := []byte(`{
            "NOT": {
                "EQ": {"field": "is_mod", "value": true},
                "ALL": {"field": "user_id", "values": ["t2_1", "t2_2", "t2_3", "t2_4"]}
            }
		}`)
		_, err := NewTargeting(targetingConfig)
		var expectedError TargetingNodeError
		if !errors.As(err, &expectedError) {
			t.Errorf("expected error %T, actual %v (%T)", expectedError, err, err)
		}
	})
}

func TestOverrideNode(t *testing.T) {
	tests := []struct {
		name           string
		targetConfig   []byte
		expectedResult bool
	}{
		{
			name:           "nominal true",
			targetConfig:   []byte(`{"OVERRIDE": true}`),
			expectedResult: true,
		},
		{
			name:           "nominal true",
			targetConfig:   []byte(`{"OVERRIDE": false}`),
			expectedResult: false,
		},
		{
			name:           "bad input non-bool",
			targetConfig:   []byte(`{"OVERRIDE": "string"}`),
			expectedResult: false,
		},
		{
			name:           "bad input object",
			targetConfig:   []byte(`{"OVERRIDE": {"key": "value"}}`),
			expectedResult: false,
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			targeting, err := NewTargeting(tt.targetConfig)
			if err != nil {
				t.Fatal(err)
			}
			result := targeting.Evaluate(inputSet())
			if result != tt.expectedResult {
				t.Errorf("expected result %t, actual %t", tt.expectedResult, result)
			}
		})
	}
}

func TestAnyNode(t *testing.T) {
	tests := []struct {
		name         string
		targetConfig []byte
		expected     bool
	}{
		{
			name: "one match",
			targetConfig: []byte(`{
				"ANY": [
					{"EQ": {"field": "num_field", "value": 5}},
					{"EQ": {"field": "str_field", "value": "str_value_1"}},
					{"EQ": {"field": "bool_field", "value": false}}
				]
			}`),
			expected: true,
		},
		{
			name:         "no match",
			targetConfig: []byte(`{"ANY": []}`),
			expected:     false,
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			targeting, err := NewTargeting(tt.targetConfig)
			if err != nil {
				t.Fatal(err)
			}
			result := targeting.Evaluate(inputSet())
			if result != tt.expected {
				t.Errorf("expected result %t, actual: %t", tt.expected, result)
			}
		})
	}
}

func TestAnyNodeInvalidInputs(t *testing.T) {
	t.Parallel()
	targetConfig := []byte(`{"ANY": {"field": "fieldname", "value": "notalist"}}`)
	_, err := NewTargeting(targetConfig)
	var expectedError TargetingNodeError
	if !errors.As(err, &expectedError) {
		t.Fatalf("expected error %T, actual: %T (%v)", expectedError, err, err)
	}
}

func TestAllNode(t *testing.T) {
	tests := []struct {
		name         string
		targetConfig []byte
		expected     bool
	}{
		{
			name: "no match",
			targetConfig: []byte(`{
				"ALL": [
					{"EQ": {"field": "num_field", "value": 6}},
					{"EQ": {"field": "str_field", "value": "str_value_1"}},
					{"EQ": {"field": "bool_field", "value": false}}
				]
			}`),
			expected: false,
		},
		{
			name: "some match",
			targetConfig: []byte(`{
				"ALL": [
					{"EQ": {"field": "num_field", "value": 5}},
					{"EQ": {"field": "str_field", "value": "str_value_1"}},
					{"EQ": {"field": "bool_field", "value": false}}
				]
			}`),
			expected: false,
		},
		{
			name: "all match",
			targetConfig: []byte(`{
				"ALL": [
					{"EQ": {"field": "num_field", "value": 5}},
					{"EQ": {"field": "str_field", "value": "string_value"}},
					{"EQ": {"field": "bool_field", "value": true}}
				]
			}`),
			expected: true,
		},
		{
			name:         "empty list",
			targetConfig: []byte(`{"ALL": []}`),
			expected:     true,
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			targeting, err := NewTargeting(tt.targetConfig)
			if err != nil {
				t.Fatal(err)
			}
			result := targeting.Evaluate(inputSet())
			if result != tt.expected {
				t.Errorf("expected result %t, actual: %t", tt.expected, result)
			}
		})
	}
}

func TestAllNodeInvalidInput(t *testing.T) {
	t.Parallel()
	targetConfig := []byte(`{"ALL": {"field": "fieldname", "value": "notalist"}}`)
	_, err := NewTargeting(targetConfig)
	var expectedError TargetingNodeError
	if !errors.As(err, &expectedError) {
		t.Fatalf("expected error %T, actual: %T (%v)", expectedError, err, err)
	}
}

func TestComparisonNode(t *testing.T) {
	tests := []struct {
		name         string
		targetConfig []byte
		expected     bool
	}{
		{
			name:         "gt node equal",
			targetConfig: []byte(`{"GT": {"field": "num_field", "value": 5}}`),
			expected:     false,
		},
		{
			name:         "gt node less than",
			targetConfig: []byte(`{"GT": {"field": "num_field", "value": 4}}`),
			expected:     true,
		},
		{
			name:         "gt node greater than",
			targetConfig: []byte(`{"GT": {"field": "num_field", "value": 6}}`),
			expected:     false,
		},
		{
			name:         "lt node equal",
			targetConfig: []byte(`{"LT": {"field": "num_field", "value": 5}}`),
			expected:     false,
		},
		{
			name:         "lt node less than",
			targetConfig: []byte(`{"LT": {"field": "num_field", "value": 4}}`),
			expected:     false,
		},
		{
			name:         "lt node greater than",
			targetConfig: []byte(`{"LT": {"field": "num_field", "value": 6}}`),
			expected:     true,
		},
		{
			name:         "ge node equal",
			targetConfig: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			expected:     true,
		},
		{
			name:         "ge node less than",
			targetConfig: []byte(`{"GE": {"field": "num_field", "value": 4}}`),
			expected:     true,
		},
		{
			name:         "ge node greater than",
			targetConfig: []byte(`{"GE": {"field": "num_field", "value": 6}}`),
			expected:     false,
		},
		{
			name:         "le node equal",
			targetConfig: []byte(`{"LE": {"field": "num_field", "value": 5}}`),
			expected:     true,
		},
		{
			name:         "le node less than",
			targetConfig: []byte(`{"LE": {"field": "num_field", "value": 4}}`),
			expected:     false,
		},
		{
			name:         "le node greater than",
			targetConfig: []byte(`{"LE": {"field": "num_field", "value": 6}}`),
			expected:     true,
		},
		{
			name:         "ne node equal",
			targetConfig: []byte(`{"NE": {"field": "num_field", "value": 5}}`),
			expected:     false,
		},
		{
			name:         "ne node less than",
			targetConfig: []byte(`{"NE": {"field": "num_field", "value": 4}}`),
			expected:     true,
		},
		{
			name:         "ne node greater than",
			targetConfig: []byte(`{"NE": {"field": "num_field", "value": 6}}`),
			expected:     true,
		},
		{
			name:         "le node explicit nil",
			targetConfig: []byte(`{"LE": {"field": "explicit_nil_field", "value": null}}`),
			expected:     false,
		},
		{
			name:         "le node implicit nil",
			targetConfig: []byte(`{"LE": {"field": "implicit nil field", "value": null}}`),
			expected:     false,
		},
		{
			name:         "gt node explicit nil",
			targetConfig: []byte(`{"GT": {"field": "explicit_nil_field", "value": null}}`),
			expected:     false,
		},
		{
			name:         "gt node implicit nil",
			targetConfig: []byte(`{"GT": {"field": "implicit nil field", "value": null}}`),
			expected:     false,
		},
		{
			name:         "ne node explicit nil",
			targetConfig: []byte(`{"NE": {"field": "explicit_nil_field", "value": null}}`),
			expected:     false,
		},
		{
			name:         "ne node implicit nil",
			targetConfig: []byte(`{"NE": {"field": "implicit nil field", "value": null}}`),
			expected:     false,
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			targeting, err := NewTargeting(tt.targetConfig)
			if err != nil {
				t.Fatal(err)
			}
			result := targeting.Evaluate(inputSet())
			if result != tt.expected {
				t.Errorf("expected result %t, actual: %t", tt.expected, result)
			}
		})
	}
}

func TestComparisonNodeBadInput(t *testing.T) {
	tests := []struct {
		name         string
		targetConfig []byte
	}{
		{
			name:         "config empty",
			targetConfig: []byte(`{"LE": {}}`),
		},
		{
			name:         "config one argument",
			targetConfig: []byte(`{"LE": {"field": "some_field"}}`),
		},
		{
			name:         "config three arguments",
			targetConfig: []byte(`{"LE": {"field": "some_field", "values": ["one", true], "value": "str_arg"}}`),
		},
		{
			name:         "config no field",
			targetConfig: []byte(`{"LE": {"fields": "some_field", "value": "str_arg"}}`),
		},
		{
			name:         "config no value",
			targetConfig: []byte(`{"LE": {"field": "some_field", "valu": "str_arg"}}`),
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable for parallel testing
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewTargeting(tt.targetConfig)
			var expectedError TargetingNodeError
			if !errors.As(err, &expectedError) {
				t.Fatalf("expected error %T, actual: %T (%v)", expectedError, err, err)
			}
		})
	}
}

func TestNumberTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		targeting []byte
		input     any
	}{
		{
			name:      "gt-node-int",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     int(5),
		},
		{
			name:      "gt-node-float64",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     float64(5),
		},
		{
			name:      "gt-node-float32",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     float32(5),
		},
		{
			name:      "gt-node-int64",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     int64(5),
		},
		{
			name:      "gt-node-int32",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     int32(5),
		},
		{
			name:      "gt-node-int16",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     int16(5),
		},
		{
			name:      "gt-node-int8",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     int8(5),
		},
		{
			name:      "gt-node-uint",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     uint(5),
		},
		{
			name:      "gt-node-uint64",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     uint64(5),
		},
		{
			name:      "gt-node-uint32",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     uint32(5),
		},
		{
			name:      "gt-node-uint16",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     uint16(5),
		},
		{
			name:      "gt-node-uint8",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     uint8(5),
		},
		{
			name:      "eq-node-int",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     int(5),
		},
		{
			name:      "eq-node-float64",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     float64(5),
		},
		{
			name:      "eq-node-float32",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     float32(5),
		},
		{
			name:      "eq-node-int64",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     int64(5),
		},
		{
			name:      "eq-node-int32",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     int32(5),
		},
		{
			name:      "eq-node-int16",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     int16(5),
		},
		{
			name:      "eq-node-int8",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     int8(5),
		},
		{
			name:      "eq-node-uint",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     uint(5),
		},
		{
			name:      "eq-node-uint64",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     uint64(5),
		},
		{
			name:      "eq-node-uint32",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     uint32(5),
		},
		{
			name:      "eq-node-uint16",
			targeting: []byte(`{"EQ": {"field": "num_field", "value": 5}}`),
			input:     uint16(5),
		},
		{
			name:      "gt-node-uint8",
			targeting: []byte(`{"GE": {"field": "num_field", "value": 5}}`),
			input:     uint8(5),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := map[string]any{
				"num_field": tt.input,
			}
			targeting, err := NewTargeting(tt.targeting)
			if err != nil {
				t.Fatal(err)
			}
			got := targeting.Evaluate(input)
			if !got {
				t.Errorf("got %t, want: %t", got, true)
			}
		})
	}
}

func inputSet() map[string]interface{} {
	inputs := make(map[string]interface{})
	inputs["bool_field"] = true
	inputs["str_field"] = "string_value"
	inputs["num_field"] = 5
	inputs["explicit_nil_field"] = nil
	return inputs
}
