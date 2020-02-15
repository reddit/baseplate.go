package experiments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type Targeting interface {
	Evaluate(inputs map[string]interface{}) bool
}

type AnyNode struct {
	children []Targeting
}

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

func (n *AnyNode) Evaluate(inputs map[string]interface{}) bool {
	for _, node := range n.children {
		if node.Evaluate(inputs) {
			return true
		}
	}
	return false
}

type AllNode struct {
	children []Targeting
}

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

func (n *AllNode) Evaluate(inputs map[string]interface{}) bool {
	for _, node := range n.children {
		if !node.Evaluate(inputs) {
			return false
		}
	}
	return true
}

type EqualNode struct {
	field  string
	values []interface{}
}

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
		field:  strings.ToLower(acceptedKey.(string)),
		values: acceptedValues,
	}, nil
}

func (n *EqualNode) Evaluate(inputs map[string]interface{}) bool {
	candidateValue := inputs[n.field]
	switch n := candidateValue.(type) {
	case int, float64:
		candidateValue = json.Number(fmt.Sprintf("%v", n))
	}
	for _, value := range n.values {
		if candidateValue == value {
			return true
		}
	}
	return false
}

type NotNode struct {
	child Targeting
}

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

func (n *NotNode) Evaluate(inputs map[string]interface{}) bool {
	return !n.child.Evaluate(inputs)
}

type OverrideNode struct {
	ReturnValue bool
}

func NewOverrideNode(inputNode interface{}) *OverrideNode {
	returnValue, ok := inputNode.(bool)
	if !ok {
		returnValue = false
	}
	return &OverrideNode{
		ReturnValue: returnValue,
	}
}

func (n *OverrideNode) Evaluate(inputs map[string]interface{}) bool {
	return n.ReturnValue
}

type ComparisonNode struct {
	field string
	value interface{}
	less  Less
}

func NewComparisonNode(inputs map[string]interface{}, less Less) (*ComparisonNode, error) {
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
		field: field.(string),
		value: value,
		less:  less,
	}, nil
}

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
		return n.less(float64(cv), value)
	case float64:
		n.less(cv, value)
	}
	return false
}

func NewTargeting(targetingConfig []byte) (Targeting, error) {
	// TODO handle empty input
	var config map[string]interface{}
	decoder := json.NewDecoder(bytes.NewBuffer(targetingConfig))
	decoder.UseNumber()
	err := decoder.Decode(&config)
	if err != nil {
		return nil, err
	}
	if len(config) != 1 {
		return nil, TargetingNodeError("call to create targeting tree expects single input key")
	}
	return parseNodes(config)
}

func parseNodes(config map[string]interface{}) (Targeting, error) {
	key := firstKeyStr(config)
	return mapOperatorNode(strings.ToLower(key), config[key])
}

func parseNode(node interface{}) (Targeting, error) {
	switch n := node.(type) {
	case []interface{}:
	case map[interface{}]interface{}:
		key := firstKey(n)
		return mapOperatorNode(strings.ToLower(key), n[key])
	case map[string]interface{}:
		key := firstKeyStr(n)
		return mapOperatorNode(strings.ToLower(key), n[key])
	}
	return nil, TargetingNodeError(fmt.Sprintf("node type %T unknown", node))
}

func firstKeyStr(m map[string]interface{}) string {
	for key := range m {
		return key
	}
	return ""
}

func firstKey(m map[interface{}]interface{}) string {
	for key := range m {
		return key.(string)
	}
	return ""
}

type Less func(float64, float64) bool

func GreaterThan(i, j float64) bool   { return i > j }
func GreaterEquals(i, j float64) bool { return i >= j }
func LessThan(i, j float64) bool      { return i < j }
func LessEquals(i, j float64) bool    { return i <= j }
func NotEqual(i, j float64) bool      { return i != j }

func mapOperatorNode(name string, value interface{}) (Targeting, error) {
	switch name {
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
		return NewComparisonNode(value.(map[string]interface{}), GreaterThan)
	case "ge":
		return NewComparisonNode(value.(map[string]interface{}), GreaterEquals)
	case "lt":
		return NewComparisonNode(value.(map[string]interface{}), LessThan)
	case "le":
		return NewComparisonNode(value.(map[string]interface{}), LessEquals)
	case "ne":
		return NewComparisonNode(value.(map[string]interface{}), NotEqual)
	}
	return nil, UnknownTargetingOperatorError(fmt.Sprintf("unrecognized operator while constructing targeting tree: %s", name))
}

type TargetingNodeError string

func (cause TargetingNodeError) Error() string {
	return string(cause)
}

type UnknownTargetingOperatorError string

func (cause UnknownTargetingOperatorError) Error() string {
	return string(cause)
}
