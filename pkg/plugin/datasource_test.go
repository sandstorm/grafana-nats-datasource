package plugin

import (
	"github.com/grafana/grafana-plugin-sdk-go/experimental"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

/*func TestQueryData(t *testing.T) {
	ds := Datasource{}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			Queries: []backend.DataQuery{
				{RefID: "A"},
			},
		},
	)
	if err != nil {
		t.Error(err)
	}

	if len(resp.Responses) != 1 {
		t.Fatal("QueryData must return a response")
	}
}*/

func TestValidJsonResponses(t *testing.T) {
	type testCase struct {
		name         string
		inputJson    string
		jqExpression string
	}

	// Testcases
	cases := []testCase{
		// simple values (all handled as string)
		{
			name:         `1_simple string`,
			inputJson:    `value1`,
			jqExpression: ``,
		},
		{
			name:         `2_object with simple values`,
			inputJson:    `{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true}`,
			jqExpression: ``,
		},
		{
			name:         `3_array with object`,
			inputJson:    `[{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true}]`,
			jqExpression: ``,
		},
		{
			name: `4_array with object with missing fields`,
			inputJson: `[
				{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
				{"s2": "my string other", "i1": 21, "f1": 12.0, "b2": false},
				{"s1": "my string other", "i1": 21, "f1": 12.0, "b1": false}
			]`,
			jqExpression: ``,
		},
		{
			name: `5_JQ expression`,
			inputJson: `[
				{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
				{"s2": "my string other", "i1": 21, "f1": 12.0}
			]`,
			jqExpression: `.[] |  {id: .i1}`,
		},
		{
			name: `6_JQ expression on optional field`,
			inputJson: `[
				{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
				{"s2": "my string other", "i1": 21, "f1": 12.0}
			]`,
			jqExpression: `.[] |  {k: .i1, id: .s1}`,
		},
		{
			name: `7_JQ expression on optional field 2`,
			inputJson: `[
				{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
				{"s2": "my string other", "i1": 21, "f1": 12.0}
			]`,
			jqExpression: `.[] |  {k: .i1, id: .s2}`,
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			res := convertJsonBytesToResponse([]byte(testcase.inputJson), testcase.jqExpression)
			AssertEqual(t, backend.StatusOK, res.Status, "res.Status")
			AssertNoError(t, res.Error)
			// TODO: make updateFile configurable
			experimental.CheckGoldenJSONResponse(t, "golden", testcase.name, &res, true)
		})
	}
}
func AssertEqual[T comparable](t *testing.T, expected, actual T, fieldName string) {
	t.Helper()

	if expected != actual {
		t.Errorf("want: %v; got: %v", expected, actual)
	}
}

func AssertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("error is non-null: %v", err)
	}
}
