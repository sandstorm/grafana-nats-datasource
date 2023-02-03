package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/experimental"
	"github.com/nats-io/nats.go"
	"github.com/sandstormmedia/nats/pkg/plugin/integration_test"
	"sync"
	"testing"
	"time"
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
type MockNatsMsgs []string

func TestValid(t *testing.T) {
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
		{
			name: `SCRIPT_1_orchestration`,
			responses: MockNatsResponsesForSubject{
				"json1": `{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true}`,
				"json2": `{"s2": "my string other", "i1": 21, "f1": 12.0}`,
			},
			q: queryModel{
				QueryType: "SCRIPT",
				JsFn: `
					const msg1 = nc.Request("json1", "", "50ms");
					const msg2 = nc.Request("json2", "", "50ms");
					const parsed1 = JSON.parse(msg1.Data);
					const parsed2 = JSON.parse(msg2.Data);
					return [parsed1, parsed2];
				`,
			},
		},
	}

	for _, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			registerNatsResponders(t, nc, testcase.responses)
			ds, pluginContext := newDatasourceForTesting()

			query, _ := json.Marshal(testcase.q)

			resp, err := ds.QueryData(
				context.Background(),
				&backend.QueryDataRequest{
					PluginContext: pluginContext,
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
			AssertNoError(t, queryResponse.Error)
			AssertEqual(t, backend.StatusOK, queryResponse.Status, "resp.Responses[0].Status")

			// TODO: make updateFile configurable
			experimental.CheckGoldenJSONResponse(t, "golden", testcase.name, &queryResponse, true)
		})
	}
}

type testCaseStreaming struct {
	name             string
	subject          string
	msgs             MockNatsMsgs
	q                queryModel
	ExpectedMessages int
}

func TestValidStreaming(t *testing.T) {
	_, nc := integration_test.StartTestNats(t)

	// Testcases
	cases := []testCaseStreaming{
		{
			name:    `SUBSCRIBE_1_simple_streaming`,
			subject: "subject1",
			msgs: MockNatsMsgs{
				`{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true}`,
				`{"s1": "my string2", "i1": 21, "f1": 21.0, "b1": false}`,
			},
			q: queryModel{
				QueryType:   "SUBSCRIBE",
				NatsSubject: "subject1",
				JsFn:        ``,
			},
			ExpectedMessages: 2,
		},
		{
			name:    `SUBSCRIBE_2_array_streaming`,
			subject: "subject1",
			msgs: MockNatsMsgs{
				`[
					{"s1": "my string", "i1": 42, "f1": 42.0, "b1": true},
					{"s2": "other"}
				]`,
				`[
					{"s1": "my string2", "i1": 21, "f1": 21.0, "b1": false},
					{"s2": "other2"}
				]`,
			},
			q: queryModel{
				QueryType:   "SUBSCRIBE",
				NatsSubject: "subject1",
				JsFn:        ``,
			},
			ExpectedMessages: 2,
		},
	}

	for i, testcase := range cases {
		t.Run(testcase.name, func(t *testing.T) {
			testcase.q.StreamRequestUuidForTesting = fmt.Sprintf("stream-%d", i)
			wg := sync.WaitGroup{}
			wg.Add(1)
			ds, pluginContext := newDatasourceForTesting()

			query, _ := json.Marshal(testcase.q)

			// send NATS messages asynchronously as soon as the datasource is listening
			go func() {
				defer wg.Done()
				waitUntilDatasourceIsListeningToStream(t, ds, testcase.q)
				for _, msg := range testcase.msgs {
					nc.Publish(testcase.subject, []byte(msg))
				}
			}()

			// send 1st message, and compare result (similar to TestValid function)
			resp, err := ds.QueryData(
				context.Background(),
				&backend.QueryDataRequest{
					PluginContext: pluginContext,
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

			AssertEqual(t, fmt.Sprintf("ds/uid1/%s", testcase.q.StreamRequestUuidForTesting), queryResponse.Frames[0].Meta.Channel, "channel does not match")
			// TODO: make updateFile configurable
			experimental.CheckGoldenJSONResponse(t, "golden", testcase.name, &queryResponse, true)

			// Now, call the streaming data source part ...
			ctx, cancel := context.WithCancel(context.Background())
			streamedMessagesChan := make(chan json.RawMessage, 100)
			wg.Add(1)
			go func() {
				defer wg.Done()

				ds.RunStream(ctx, &backend.RunStreamRequest{
					PluginContext: backend.PluginContext{},
					Path:          testcase.q.StreamRequestUuidForTesting,
					Data:          nil,
				}, backend.NewStreamSender(&customPacketSender{
					c: streamedMessagesChan,
				}))
			}()

			// ... and validate the streamed messages
			readAndValidateAllStreamedMessages(t, testcase, streamedMessagesChan)

			// cleanup
			cancel()  // cancel ds.RunStream()
			wg.Wait() // wait until all goroutines for the testcase are finished
		})
	}

}

func readAndValidateAllStreamedMessages(t *testing.T, testcase testCaseStreaming, streamedMessagesChan chan json.RawMessage) {
	// we already received a message during setup, that's why we start counting at 1.
	for i := 1; i < testcase.ExpectedMessages; i++ {
		select {
		case msg := <-streamedMessagesChan:
			var streamedFrame data.Frame
			err := json.Unmarshal(msg, &streamedFrame)
			if err != nil {
				t.Fatalf("could not unmarshal streaming message %d: %s", i, err)
				return
			}

			experimental.CheckGoldenJSONFrame(t, "golden", fmt.Sprintf("%s_s%d", testcase.name, i), &streamedFrame, true)
		case <-time.After(time.Second):
			t.Fatalf("timeout receiving message %d", i)
		}
	}
}

type customPacketSender struct {
	c chan<- json.RawMessage
}

func (s *customPacketSender) Send(packet *backend.StreamPacket) error {
	s.c <- packet.Data
	return nil
}

func waitUntilDatasourceIsListeningToStream(t *testing.T, ds *Datasource, q queryModel) {
	// we at most wait 1 second.
	for i := 0; i < 100; i++ {
		sr := ds.streamResponsesSoFar.Get(q.StreamRequestUuidForTesting)
		if sr != nil && sr.Value().subscribed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Datasource is not listening to stream after 1 second")

}

func newDatasourceForTesting() (*Datasource, backend.PluginContext) {
	dsTmp, _ := NewDatasource(backend.DataSourceInstanceSettings{
		UID: "uid1",
	})
	ds := dsTmp.(*Datasource)

	dsOpts, _ := json.Marshal(MyDataSourceOptions{
		NatsUrl:        fmt.Sprintf("127.0.0.1:%d", integration_test.TEST_PORT),
		Authentication: "NONE",
	})
	dsOptsSecure := map[string]string{}

	return ds, backend.PluginContext{
		DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{
			JSONData:                dsOpts,
			DecryptedSecureJSONData: dsOptsSecure,
		},
	}
}

func registerNatsResponders(t *testing.T, nc *nats.Conn, responses MockNatsResponsesForSubject) {
	for subject, resp := range responses {
		respCopy := resp // because we need the value in a closure
		subj, _ := nc.Subscribe(subject, func(msg *nats.Msg) {
			msg.Respond([]byte(respCopy))
		})
		t.Cleanup(func() {
			subj.Unsubscribe()
		})
	}
}
func AssertEqual[T comparable](t *testing.T, expected, actual T, fieldName string) {
	t.Helper()

	if expected != actual {
		t.Fatalf("want: %v; got: %v", expected, actual)
	}
}

func AssertNoError(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("error is non-null: %v", err)
	}
}
