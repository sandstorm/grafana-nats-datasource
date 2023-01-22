package plugin

import (
	"github.com/itchyny/gojq"
)

func processViaGojq(v interface{}, jqExpression string) ([]interface{}, error) {
	query, err := gojq.Parse(jqExpression)
	if err != nil {
		return nil, err
	}
	iter := query.Run(v) // or query.RunWithContext

	var elements []interface{}
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}

		elements = append(elements, v)
	}

	return elements, nil
}
