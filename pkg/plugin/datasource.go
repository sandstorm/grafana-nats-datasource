package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"os"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
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

type queryModel struct {
	Subject string
	Data    []byte
	Timeout time.Duration
}

func (d *Datasource) query(_ context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
	//////////////
	// 1) Data Source option loading
	//////////////
	var dataSourceOptions *MyDataSourceOptions
	err := json.Unmarshal(pCtx.DataSourceInstanceSettings.JSONData, &dataSourceOptions)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "data source options json unmarshal: "+err.Error())
	}
	var dataSourceSecureOptions *MySecureJsonData
	secureBytes, err := json.Marshal(pCtx.DataSourceInstanceSettings.DecryptedSecureJSONData)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "decrypted secureJson could not be converted to JSON: "+err.Error())
	}
	err = json.Unmarshal(secureBytes, &dataSourceSecureOptions)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "decrypted secureJson could not be parsed: "+err.Error())
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
	var response backend.DataResponse

	// Unmarshal the JSON into our queryModel.
	var qm queryModel

	err = json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "json unmarshal: "+err.Error())
	}

	if qm.Timeout == 0 {
		qm.Timeout = 5 * time.Second
	}
	_, err = natsConn.Request(qm.Subject, qm.Data, qm.Timeout)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "NATS request error: "+err.Error())
	}
	// resp.Data

	// create data frame response.
	frame := data.NewFrame("response")

	// add fields.
	frame.Fields = append(frame.Fields,
		data.NewField("time", nil, []time.Time{query.TimeRange.From, query.TimeRange.To}),
		data.NewField("values", nil, []int64{10, 20}),
	)

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

	var status = backend.HealthStatusOk
	var message = "Data source is working"

	/*if rand.Int()%2 == 0 {
		status = backend.HealthStatusError
		message = "randomized error"
	}*/

	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
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
				kp, err := nkeys.FromSeed(secureOptions.NkeySeed)
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
		_, err = file.Write(secureOptions.Jwt)
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
