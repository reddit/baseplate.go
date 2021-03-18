package healthcheck

import (
	"flag"
	"fmt"
	"sort"
	"strings"
)

type oneof struct {
	choices map[string]interface{}
	value   string
}

var _ flag.Getter = (*oneof)(nil)

func (o *oneof) String() string {
	return o.value
}

func (o *oneof) Get() interface{} {
	return o
}

func (o *oneof) Set(v string) error {
	if _, ok := o.choices[v]; ok {
		o.value = v
		return nil
	}
	return fmt.Errorf("%q is not one of the choices of %s", v, o.choicesString())
}

func (o oneof) choicesString() string {
	// Sort the map to stabilize the output
	choices := make([]string, 0, len(o.choices))
	for c := range o.choices {
		choices = append(choices, c)
	}
	sort.Strings(choices)

	var sb strings.Builder
	sb.WriteString("(")
	for i, c := range choices {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", c))
	}
	sb.WriteString(")")
	return sb.String()
}

func (o oneof) getValue() interface{} {
	return o.choices[o.value]
}
