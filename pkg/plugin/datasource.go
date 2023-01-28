package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/live"
	"github.com/jellydator/ttlcache/v3"
	"github.com/nats-io/nats.go"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces- only those which are required for a particular task.
var (
	_ backend.QueryDataHandler   = (*Datasource)(nil)
	_ backend.CheckHealthHandler = (*Datasource)(nil)

	// TODO: https://grafana.com/tutorials/build-a-streaming-data-source-plugin/
	_ backend.StreamHandler         = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// NewDatasource creates a new datasource instance.
func NewDatasource(config backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &Datasource{
		uid: config.UID,
	}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct {
	uid      string
	streamResponsesSoFar ttlcache.Cache[string,] ttlcache.New[string, string]()
	streamMu sync.RWMutex
	streams  map[string]models.TwinMakerQuery
}

type streamResponse {

}

func (ds *Datasource) SubscribeStream(ctx context.Context, request *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	status := backend.SubscribeStreamStatusNotFound

	ds.streamMu.RLock()
	if _, ok := ds.streams[request.Path]; ok {
		status = backend.SubscribeStreamStatusOK
	}
	ds.streamMu.RUnlock()

	return &backend.SubscribeStreamResponse{
		Status: status,
	}, nil
}

func (ds *Datasource) PublishStream(ctx context.Context, request *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	return &backend.PublishStreamResponse{
		Status: backend.PublishStreamStatusPermissionDenied,
	}, nil
}

func (ds *Datasource) RunStream(ctx context.Context, request *backend.RunStreamRequest, sender *backend.StreamSender) error {
	sender.SendFrame(frame, data.IncludeAll)
	//TODO implement me
	panic("implement me")
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (ds *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (ds *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// when logging at a non-Debug level, make sure you don't include sensitive information in the message
	// (like the *backend.QueryDataRequest)
	log.DefaultLogger.Debug("QueryData called", "numQueries", len(req.Queries))

	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := ds.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

func (ds *Datasource) loadDataSourceOptions(pCtx backend.PluginContext) (*MyDataSourceOptions, *MySecureJsonData, error) {
	var dataSourceOptions *MyDataSourceOptions
	err := json.Unmarshal(pCtx.DataSourceInstanceSettings.JSONData, &dataSourceOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("data source options json unmarshal: %w", err)
	}
	var dataSourceSecureOptions *MySecureJsonData
	secureBytes, err := json.Marshal(pCtx.DataSourceInstanceSettings.DecryptedSecureJSONData)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypted secureJson could not be converted to JSON: %w", err)
	}

	err = json.Unmarshal(secureBytes, &dataSourceSecureOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypted secureJson could not be parsed: %w", err)
	}
	return dataSourceOptions, dataSourceSecureOptions, nil
}

func (ds *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	//////////////
	// 1) Data Source option loading
	//////////////
	dataSourceOptions, dataSourceSecureOptions, err := ds.loadDataSourceOptions(pCtx)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "data source could not be loaded: "+err.Error())
	}

	//////////////
	// 2) Connect
	//////////////
	natsConn, err := ds.connectNats(dataSourceOptions, dataSourceSecureOptions)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "NATS connection error:  "+err.Error())
	}
	// TODO: lateron, keep the nats connection open for some minutes instead of tearing it down for every req.
	defer natsConn.Close()

	//////////////
	// 3) do request
	//////////////
	// Unmarshal the JSON into our queryModel.
	var qm queryModel

	err = json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "json unmarshal: "+err.Error())
	}

	if qm.RequestTimeout.Duration == 0 {
		qm.RequestTimeout.Duration = 5 * time.Second
	}
	if qm.QueryType == QueryTypeRequestReply {
		resp, err := natsConn.Request(qm.NatsSubject, []byte(qm.RequestData), qm.RequestTimeout.Duration)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, "NATS request error: "+err.Error())
		}

		frame, err := convertJsonBytesToResponse(resp.Data, qm.JqExpression)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, "Response conversion error: "+err.Error())
		}

		return backend.DataResponse{
			Frames: data.Frames{
				frame,
			},
			Status: backend.StatusOK,
		}
	} else if qm.QueryType == QueryTypeRequestMultireplyStreaming {
		return ds.requestMultireplyStreaming(qm, query, pCtx, natsConn)
	} else {
		return backend.ErrDataResponse(backend.StatusBadRequest, "Invalid Query Type: "+qm.QueryType)
	}
}

func convertJsonBytesToResponse(respData []byte, jqExpression string) (*data.Frame, error) {
	// Get slice of data with optional leading whitespace removed.
	// See RFC 7159, Section 2 for the definition of JSON whitespace.
	x := bytes.TrimLeft(respData, " \t\r\n")
	isArray := len(x) > 0 && x[0] == '['
	isObject := len(x) > 0 && x[0] == '{'
	var frame *data.Frame
	var err error
	log.DefaultLogger.Info(fmt.Sprintf("Converting JSON bytes to response. isArray=%v, isObj=%v", isArray, isObject))

	if isArray {
		if jqExpression == "" {
			// pass all array values (even single-value ones) through JQ - to simplify code paths.
			jqExpression = ".[]"
		}
		var v []interface{}
		if err := json.Unmarshal(respData, &v); err != nil {
			return nil, fmt.Errorf("JSON could not be parsed (1): %w", err)
		}
		frame, err = processViaGojq(v, jqExpression)
		if err != nil {
			return nil, fmt.Errorf("JQ could not process array: %w", err)
		}
	} else if isObject {
		if jqExpression == "" {
			// pass all object values through JQ - to simplify code paths.
			jqExpression = "."
		}
		var v interface{}
		if err := json.Unmarshal(respData, &v); err != nil {
			return nil, fmt.Errorf("JSON could not be parsed (2): %w", err)
		}
		frame, err = processViaGojq(v, jqExpression)
		if err != nil {
			log.DefaultLogger.Error(fmt.Sprintf("Error processing object: %s", err))
			return nil, fmt.Errorf("JQ could not process object: %w", err)
		}
	} else {
		// not an array nor an object. Respond with a single-column "result" string dataframe.
		frame = data.NewFrame("response")
		frame.Fields = append(frame.Fields, data.NewField(data.TimeSeriesValueFieldName, nil, []string{string(respData)}))
	}

	return frame, nil
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (ds *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	// when logging at a non-Debug level, make sure you don't include sensitive information in the message
	// (like the *backend.QueryDataRequest)
	log.DefaultLogger.Debug("CheckHealth called")

	//////////////
	// 1) Data Source option loading
	//////////////
	dataSourceOptions, dataSourceSecureOptions, err := ds.loadDataSourceOptions(req.PluginContext)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Data source options could not be loaded (should never happen)" + err.Error(),
		}, nil
	}

	//////////////
	// 2) Connect
	//////////////
	natsConn, err := ds.connectNats(dataSourceOptions, dataSourceSecureOptions)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "NATS could not be connected to: " + err.Error(),
		}, nil
	}
	natsConn.Close()

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source is working",
	}, nil
}

func (ds *Datasource) requestMultireplyStreaming(qm queryModel, query backend.DataQuery, pCtx backend.PluginContext, natsConn *nats.Conn) backend.DataResponse {
	requestUuid := uuid.NewString()
	frame := data.NewFrame("response")
	channel := live.Channel{
		Scope:     live.ScopeDatasource,
		Namespace: ds.uid,
		Path:      requestUuid,
	} // !! https://github.com/grafana/grafana-iot-twinmaker-app/blob/0947ce1ff0afec8372cae624566726e68687137b/pkg/plugin/datasource.go
	// https://github.com/grafana/mqtt-datasource/blob/main/pkg/plugin/datasource.go
	frame.SetMeta(&data.FrameMeta{Channel: channel.String()})

	// we manually need
	respInbox := natsConn.NewInbox()
	responsesSoFar := make([][]byte, 0, 10)
	natsConn.Subscribe(respInbox, func(msg *nats.Msg) {
		// totally inefficient: for every new message, we parse the FULL response (with all already existing messages).
		// TODO: this should be made more intelligent later :D
		responsesSoFar = append(responsesSoFar, msg.Data)
		var r = []byte("[")
		r = append(r, bytes.Join(responsesSoFar, []byte(","))...)
		r = append(r, []byte("]")...)

		resp, _ := convertJsonBytesToResponse(r, qm.JqExpression)
		ds.streamMu.Lock()
		ds.streams[requestUuid] = resp
		ds.streamMu.Unlock()
	})

	return backend.DataResponse{
		Frames: data.Frames{
			frame,
		},
		Status: backend.StatusOK,
	}
}
