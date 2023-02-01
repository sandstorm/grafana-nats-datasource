package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/experimental"
	"github.com/nats-io/nats.go"
	"github.com/sandstormmedia/nats/pkg/plugin/integration_test"
	"testing"
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

type MockNatsResponsesForSubject map[string]string

func TestValidJsonResponses(t *testing.T) {
	_, nc := integration_test.StartTestNats(t)

	type testCase struct {
		name      string
		responses MockNatsResponsesForSubject
		q         queryModel
	}

	// Testcases
	cases := []testCase{
		{
			name: `REQUEST_REPLY_1_object with simple values`,
			responses: MockNatsResponsesForSubject{
				"json": `{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true}`,
			},
			q: queryModel{
				QueryType:   "REQUEST_REPLY",
				NatsSubject: "json",
				JsFn:        ``,
			},
		},
		{
			name: `REQUEST_REPLY_2_array with object`,
			responses: MockNatsResponsesForSubject{
				"jsonArr": `[{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true}]`,
			},
			q: queryModel{
				QueryType:   "REQUEST_REPLY",
				NatsSubject: "jsonArr",
				JsFn:        ``,
			},
		},
		{
			name: `REQUEST_REPLY_3_array with object with missing fields`,
			responses: MockNatsResponsesForSubject{
				"jsonArr": `[
					{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
					{"s2": "my string other", "i1": 21, "f1": 12.0, "b2": false},
					{"s1": "my string other", "i1": 21, "f1": 12.0, "b1": false}
				]`,
			},
			q: queryModel{
				QueryType:   "REQUEST_REPLY",
				NatsSubject: "jsonArr",
				JsFn:        ``,
			},
		},
		{
			name: `REQUEST_REPLY_4_JS expression`,
			responses: MockNatsResponsesForSubject{
				"jsonArr": `[
					{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
					{"s2": "my string other", "i1": 21, "f1": 12.0}
				]`,
			},
			q: queryModel{
				QueryType:   "REQUEST_REPLY",
				NatsSubject: "jsonArr",
				JsFn: `
					const parsed = JSON.parse(msg.Data);
					return parsed.map((row) => ({id: row.i1}));
				`,
			},
		},
		{
			name: `REQUEST_REPLY_5_JS expression on optional field`,
			responses: MockNatsResponsesForSubject{
				"jsonArr": `[
					{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
					{"s2": "my string other", "i1": 21, "f1": 12.0}
				]`,
			},
			q: queryModel{
				QueryType:   "REQUEST_REPLY",
				NatsSubject: "jsonArr",
				JsFn: `
					const parsed = JSON.parse(msg.Data);
					return parsed.map((row) => ({k: row.i1, id: row.s1}));
				`,
			},
		},
		{
			name: `REQUEST_REPLY_6_JS expression on optional field 2`,
			responses: MockNatsResponsesForSubject{
				"jsonArr": `[
					{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
					{"s2": "my string other", "i1": 21, "f1": 12.0}
				]`,
			},
			q: queryModel{
				QueryType:   "REQUEST_REPLY",
				NatsSubject: "jsonArr",
				JsFn: `
					const parsed = JSON.parse(msg.Data);
					return parsed.map((row) => ({k: row.i1, id: row.s2}));
				`,
			},
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			// Register all NATS responders as configured.
			for subject, resp := range testcase.responses {
				respCopy := resp // because we need the value in a closure
				subj, _ := nc.Subscribe(subject, func(msg *nats.Msg) {
					msg.Respond([]byte(respCopy))
				})
				t.Cleanup(func() {
					subj.Unsubscribe()
				})
			}
			dsTmp, _ := NewDatasource(backend.DataSourceInstanceSettings{
				UID: "uid1",
			})
			ds := dsTmp.(*Datasource)

			dsOpts, _ := json.Marshal(MyDataSourceOptions{
				NatsUrl:        fmt.Sprintf("127.0.0.1:%d", integration_test.TEST_PORT),
				Authentication: "NONE",
			})
			dsOptsSecure := map[string]string{}

			query, _ := json.Marshal(testcase.q)

			resp, err := ds.QueryData(
				context.Background(),
				&backend.QueryDataRequest{
					PluginContext: backend.PluginContext{
						DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
							JSONData:                dsOpts,
							DecryptedSecureJSONData: dsOptsSecure,
						},
					},
					Queries: []backend.DataQuery{
						{
							RefID: "A",
							JSON:  query,
						},
					},
				},
			)
			AssertNoError(t, err)
			queryResponse := resp.Responses["A"]
			AssertEqual(t, backend.StatusOK, queryResponse.Status, "resp.Responses[0].Status")
			AssertNoError(t, queryResponse.Error)

			// TODO: make updateFile configurable
			experimental.CheckGoldenJSONResponse(t, "golden", testcase.name, &queryResponse, true)
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
