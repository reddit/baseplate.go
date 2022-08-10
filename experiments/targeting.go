package experiments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Targeting is the common interface to implement experiment targeting.
// Evaluated whether the provided input matches the expected values.
type Targeting interface {
	Evaluate(inputs map[string]interface{}) bool
}

// NewTargeting parses the given targeting configuration into a Targeting.
func NewTargeting(targetingConfig []byte) (Targeting, error) {
	var config map[string]interface{}
	decoder := json.NewDecoder(bytes.NewBuffer(targetingConfig))
	// ensures numbers are parsed into json.Number in order to differentiate
	// between integer and float64 later on
	decoder.UseNumber()
	err := decoder.Decode(&config)
	if err != nil {
		return nil, err
	}
	if len(config) != 1 {
		return nil, TargetingNodeError("call to create targeting tree expects single input key")
	}
	return parseNode(config)
}

func parseNode(node interface{}) (Targeting, error) {
	switch n := node.(type) {
	case map[interface{}]interface{}:
		key, err := firstKey(n)
		if err != nil {
			return nil, err
		}
		return mapOperatorNode(key, n[key])
	case map[string]interface{}:
		key, err := firstKey(n)
		if err != nil {
			return nil, err
		}
		return mapOperatorNode(key, n[key])
	}
	return nil, TargetingNodeError(fmt.Sprintf("node type %T unknown", node))
}

func firstKey(m interface{}) (string, error) {
	switch n := m.(type) {
	case map[string]interface{}:
		for key := range n {
			return key, nil
		}
	case map[interface{}]interface{}:
		for key := range n {
			k, ok := key.(string)
			if !ok {
				return "", TargetingNodeError(fmt.Sprintf("unknown key type %T", key))
			}
			return k, nil
		}
	}
	return "", TargetingNodeError(fmt.Sprintf("unknown parsed node type %T", m))
}

// AnyNode evaluates to true if at least one child node returns true.
type AnyNode struct {
	children []Targeting
}

// NewAnyNode parses the underlying input into an AnyNode.
func NewAnyNode(input interface{}) (Targeting, error) {
	inputNodes, ok := input.([]interface{})
	if !ok {
		return nil, TargetingNodeError("input to AnyNode expects an array")
	}
	children := make([]Targeting, len(inputNodes))
	for i, node := range inputNodes {
		node, err := parseNode(node)
		if err != nil {
			return nil, err
		}
		children[i] = node
	}
	return &AnyNode{
		children: children,
	}, nil
}

// Evaluate returns true if at least one child node returns true.
func (n *AnyNode) Evaluate(inputs map[string]interface{}) bool {
	for _, node := range n.children {
		if node.Evaluate(inputs) {
			return true
		}
	}
	return false
}

// AllNode evaluates to true if all child nodes returns true.
type AllNode struct {
	children []Targeting
}

// NewAllNode parses the underlying input into an AllNode.
func NewAllNode(input interface{}) (Targeting, error) {
	inputNodes, ok := input.([]interface{})
	if !ok {
		return nil, TargetingNodeError("input to AnyNode expects an array")
	}
	children := make([]Targeting, len(inputNodes))
	for i, node := range inputNodes {
		node, err := parseNode(node)
		if err != nil {
			return nil, err
		}
		children[i] = node
	}
	return &AllNode{
		children: children,
	}, nil
}

// Evaluate returns true if all child nodes returns true.
func (n *AllNode) Evaluate(inputs map[string]interface{}) bool {
	for _, node := range n.children {
		if !node.Evaluate(inputs) {
			return false
		}
	}
	return true
}

// EqualNode is used to determine whether an attribute equals a single value or
// a value in a list.
//
// A full EqualNode in a targeting tree configuration looks like this:
//
//	{
//	   EQ: {
//	        field: <field_name>
//	        value: <accepted_value>
//	    }
//	}
//
// The expected input to this constructor from the above example would be::
//
//	{
//	    field: <field_name>,
//	    value: <accepted_value>
//	}
type EqualNode struct {
	fieldName      string
	acceptedValues []interface{}
}

// NewEqualNode parses the underlying input into an EqualNode.
func NewEqualNode(inputNodes map[string]interface{}) (Targeting, error) {
	if len(inputNodes) != 2 {
		return nil, TargetingNodeError("EqualNode expects exactly two fields")
	}
	acceptedKey, ok := inputNodes["field"]
	if !ok {
		return nil, TargetingNodeError("EqualNode expects input key 'field'")
	}

	value, valueOK := inputNodes["value"]
	values, valuesOK := inputNodes["values"]
	if !valueOK && !valuesOK {
		return nil, TargetingNodeError("EqualNode expects input key 'value' or 'values'.")
	}

	var acceptedValues []interface{}
	if valuesOK {
		acceptedValues = values.([]interface{})
	} else if valueOK {
		acceptedValues = []interface{}{value}
	}
	return &EqualNode{
		fieldName:      strings.ToLower(acceptedKey.(string)),
		acceptedValues: acceptedValues,
	}, nil
}

// Evaluate returns true if the given attribute has the expected value.
func (n *EqualNode) Evaluate(inputs map[string]interface{}) bool {
	candidateValue := inputs[n.fieldName]
	switch n := candidateValue.(type) {
	case int, float64:
		candidateValue = json.Number(fmt.Sprintf("%v", n))
	}
	for _, value := range n.acceptedValues {
		if candidateValue == value {
			return true
		}
	}
	return false
}

// NotNode is a boolean 'not' operator and negates the child node.
type NotNode struct {
	child Targeting
}

// NewNotNode parses the underlying input into an NotNode.
func NewNotNode(inputNodes map[string]interface{}) (*NotNode, error) {
	if len(inputNodes) != 1 {
		return nil, TargetingNodeError("NotNode expects exactly one field")
	}
	node, err := parseNode(inputNodes)
	if err != nil {
		return nil, err
	}
	return &NotNode{
		child: node,
	}, nil
}

// Evaluate returns the negation of the child's evaluation.
func (n *NotNode) Evaluate(inputs map[string]interface{}) bool {
	return !n.child.Evaluate(inputs)
}

// OverrideNode is an override to the targeting and can always return true or
// false.
type OverrideNode struct {
	ReturnValue bool
}

// NewOverrideNode parses the underlying input into an OverrideNode.
func NewOverrideNode(inputNode interface{}) *OverrideNode {
	returnValue, ok := inputNode.(bool)
	if !ok {
		returnValue = false
	}
	return &OverrideNode{
		ReturnValue: returnValue,
	}
}

// Evaluate returns the configured boolean return value.
func (n *OverrideNode) Evaluate(inputs map[string]interface{}) bool {
	return n.ReturnValue
}

// ComparisonNode is a non-equality comparison operators (gt, ge, lt, le).
//
// Expects as input the input node as well as an operator (from the operator
// module). Operator must be one that expects two inputs ( ie: gt, ge, lt, le,
// eq, ne).
type ComparisonNode struct {
	field    string
	value    interface{}
	comparer less
}

// NewComparisonNode parses the underlying input into an ComparisonNode.
func NewComparisonNode(inputs map[string]interface{}, comparer less) (*ComparisonNode, error) {
	if len(inputs) != 2 {
		return nil, TargetingNodeError("ComparisonNode expects exactly two fields")
	}
	field, ok := inputs["field"]
	if !ok {
		return nil, TargetingNodeError("ComparisonNode expects input key 'field'.")
	}
	value, ok := inputs["value"]
	if !ok {
		return nil, TargetingNodeError("ComparisonNode expects input key 'value'.")
	}
	return &ComparisonNode{
		field:    field.(string),
		value:    value,
		comparer: comparer,
	}, nil
}

// Evaluate returns true if the comparison holds true and false otherwise.
func (n *ComparisonNode) Evaluate(inputs map[string]interface{}) bool {
	candidateValue := inputs[n.field]
	if n.value == nil {
		return false
	}
	value, err := (n.value.(json.Number)).Float64()
	if err != nil {
		return false
	}
	switch cv := candidateValue.(type) {
	case int:
		return n.comparer(float64(cv), value)
	case float64:
		n.comparer(cv, value)
	}
	return false
}

type less func(float64, float64) bool

func greaterThan(i, j float64) bool   { return i > j }
func greaterEquals(i, j float64) bool { return i >= j }
func lessThan(i, j float64) bool      { return i < j }
func lessEquals(i, j float64) bool    { return i <= j }
func notEqual(i, j float64) bool      { return i != j }

func mapOperatorNode(operator string, value interface{}) (Targeting, error) {
	operator = strings.ToLower(operator)
	switch operator {
	case "any":
		return NewAnyNode(value)
	case "all":
		return NewAllNode(value)
	case "eq":
		return NewEqualNode(value.(map[string]interface{}))
	case "not":
		return NewNotNode(value.(map[string]interface{}))
	case "override":
		return NewOverrideNode(value), nil
	case "gt":
		return NewComparisonNode(value.(map[string]interface{}), greaterThan)
	case "ge":
		return NewComparisonNode(value.(map[string]interface{}), greaterEquals)
	case "lt":
		return NewComparisonNode(value.(map[string]interface{}), lessThan)
	case "le":
		return NewComparisonNode(value.(map[string]interface{}), lessEquals)
	case "ne":
		return NewComparisonNode(value.(map[string]interface{}), notEqual)
	}
	return nil, UnknownTargetingOperatorError(operator)
}

// TargetingNodeError is returned when there was an inconsistency in the
// targeting due to operator mismatch or violation of their properties in the
// input.
type TargetingNodeError string

func (cause TargetingNodeError) Error() string {
	return "experiments: " + string(cause)
}

// UnknownTargetingOperatorError is returned when the parsed operator is not
// known.
type UnknownTargetingOperatorError string

func (operator UnknownTargetingOperatorError) Error() string {
	return "experiments: unrecognized operator while constructing targeting tree: " + string(operator)
}
