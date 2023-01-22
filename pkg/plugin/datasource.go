package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/framestruct"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"os"
	"strings"
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
	// _ backend.StreamHandler         = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// NewDatasource creates a new datasource instance.
func NewDatasource(_ backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &Datasource{}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct{}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// when logging at a non-Debug level, make sure you don't include sensitive information in the message
	// (like the *backend.QueryDataRequest)
	log.DefaultLogger.Debug("QueryData called", "numQueries", len(req.Queries))

	// create response struct
	response := backend.NewQueryDataResponse()

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, req.PluginContext, q)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

func (d *Datasource) loadDataSourceOptions(pCtx backend.PluginContext) (*MyDataSourceOptions, *MySecureJsonData, error) {
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

func (d *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	//////////////
	// 1) Data Source option loading
	//////////////
	dataSourceOptions, dataSourceSecureOptions, err := d.loadDataSourceOptions(pCtx)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "data source could not be loaded: "+err.Error())
	}

	//////////////
	// 2) Connect
	//////////////
	natsConn, err := d.connectNats(dataSourceOptions, dataSourceSecureOptions)
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

	if qm.RequestTimeout == 0 {
		qm.RequestTimeout = 5 * time.Second
	}
	if qm.QueryType == QueryTypeRequestReply {
		resp, err := natsConn.Request(qm.NatsSubject, []byte(qm.RequestData), qm.RequestTimeout)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, "NATS request error: "+err.Error())
		}

		return convertJsonBytesToResponse(resp.Data, qm.JqExpression)
	} else {
		return backend.ErrDataResponse(backend.StatusBadRequest, "Invalid Query Type: "+qm.QueryType)
	}
}

func convertJsonBytesToResponse(respData []byte, jqExpression string) backend.DataResponse {
	var response backend.DataResponse

	// Get slice of data with optional leading whitespace removed.
	// See RFC 7159, Section 2 for the definition of JSON whitespace.
	x := bytes.TrimLeft(respData, " \t\r\n")
	isArray := len(x) > 0 && x[0] == '['
	isObject := len(x) > 0 && x[0] == '{'
	var frame *data.Frame
	var err error

	if isArray {
		if strings.TrimSpace(jqExpression) != "" {
			var v []interface{}
			if err := json.Unmarshal(respData, &v); err != nil {
				return backend.ErrDataResponse(backend.StatusBadRequest, "JSON could not be parsed: "+err.Error())
			}
			v, err := processViaGojq(v, jqExpression)
			if err != nil {
				return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("JQ transfer did not work: %s", err.Error()))
			}
			frame, err = framestruct.ToDataFrame("response", v)
		} else {
			var v []map[string]interface{}
			if err := json.Unmarshal(respData, &v); err != nil {
				return backend.ErrDataResponse(backend.StatusBadRequest, "JSON could not be parsed: "+err.Error())
			}
			frame, err = framestruct.ToDataFrame("response", v)
		}
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("JSON could not be converted to DataFrame: %s", err.Error()))
		}
	} else if isObject {
		if strings.TrimSpace(jqExpression) != "" {
			var v interface{}
			if err := json.Unmarshal(respData, &v); err != nil {
				return backend.ErrDataResponse(backend.StatusBadRequest, "JSON could not be parsed: "+err.Error())
			}
			v, err := processViaGojq(v, jqExpression)
			if err != nil {
				return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("JQ transfer did not work: %s", err.Error()))
			}
			frame, err = framestruct.ToDataFrame("response", v)
		} else {
			var v map[string]interface{}
			if err := json.Unmarshal(respData, &v); err != nil {
				return backend.ErrDataResponse(backend.StatusBadRequest, "JSON could not be parsed: "+err.Error())
			}
			frame, err = framestruct.ToDataFrame("response", v)
		}
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("JSON could not be converted to DataFrame: %s", err.Error()))
		}
	} else {
		// not an array nor an object. Respond with a single-column "result" string dataframe.
		frame = data.NewFrame("response")
		frame.Fields = append(frame.Fields, data.NewField("response", nil, []json.RawMessage{respData}))
	}

	// add the frames to the response.
	response.Frames = append(response.Frames, frame)

	return response
}

// CheckHealth handles health checks sent from Grafana to the plugin.
// The main use case for these health checks is the test button on the
// datasource configuration page which allows users to verify that
// a datasource is working as expected.
func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	// when logging at a non-Debug level, make sure you don't include sensitive information in the message
	// (like the *backend.QueryDataRequest)
	log.DefaultLogger.Debug("CheckHealth called")

	//////////////
	// 1) Data Source option loading
	//////////////
	dataSourceOptions, dataSourceSecureOptions, err := d.loadDataSourceOptions(req.PluginContext)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "Data source options could not be loaded (should never happen)" + err.Error(),
		}, nil
	}

	//////////////
	// 2) Connect
	//////////////
	natsConn, err := d.connectNats(dataSourceOptions, dataSourceSecureOptions)
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

func (d *Datasource) connectNats(options *MyDataSourceOptions, secureOptions *MySecureJsonData) (*nats.Conn, error) {
	var natsConn *nats.Conn
	var err error
	if options.Authentication == AuthenticationNone {
		natsConn, err = nats.Connect(options.NatsUrl)
	} else if options.Authentication == AuthenticationNkey {
		natsConn, err = nats.Connect(options.NatsUrl, nats.Nkey(
			options.Nkey,
			func(nonce []byte) ([]byte, error) {
				kp, err := nkeys.FromSeed([]byte(secureOptions.NkeySeed))
				if err != nil {
					return nil, fmt.Errorf("unable to load key pair from NkeySeed: %w", err)
				}
				// Wipe our key on exit.
				defer kp.Wipe()

				sig, _ := kp.Sign(nonce)
				return sig, nil
			},
		))
	} else if options.Authentication == AuthenticationUserPass {
		natsConn, err = nats.Connect(options.NatsUrl, nats.UserInfo(options.Username, secureOptions.Password))
	} else if options.Authentication == AuthenticationJWT {
		// WORKAROUND: store credentials in a temp-file
		file, err := os.CreateTemp("", "tmp-jwt")
		if err != nil {
			return nil, fmt.Errorf("TODO: %w", err)
		}
		defer os.Remove(file.Name())
		_, err = file.Write([]byte(secureOptions.Jwt))
		if err != nil {
			return nil, fmt.Errorf("TODO: %w", err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("TODO: %w", err)
		}

		natsConn, err = nats.Connect(options.NatsUrl, nats.UserCredentials(file.Name()))
	} else {
		// TODO: TOKEN AUTH
		return nil, fmt.Errorf("TODO")
	}

	return natsConn, err
}
