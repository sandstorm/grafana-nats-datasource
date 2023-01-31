package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/live"
	"github.com/jellydator/ttlcache/v3"
	"github.com/nats-io/nats.go"
	"github.com/sandstormmedia/nats/pkg/plugin/framestruct"
	"github.com/sandstormmedia/nats/pkg/plugin/goja"
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
		uid:                  config.UID,
		natsConnOnce:         new(sync.Once),
		streamResponsesSoFar: ttlcache.New[string, *streamResponse](),
	}, nil
}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (ds *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct {
	uid                  string
	streamResponsesSoFar *ttlcache.Cache[string, *streamResponse]

	// natsConnOnce is implementation detail of connectNats to ensure we only create one NATS connection without any race conditions
	natsConnOnce *sync.Once
	// natsConn contains the singleton NATS connection for the datasource. Never access this directly, but always use connectNats.
	natsConn *nats.Conn
	// natsConnErr contains the error if creating the NATS connection failed. Never access this directly, but always use connectNats.
	natsConnErr error
}

type streamResponse struct {
	onNewMessages          chan bool
	currentFrame           *data.Frame
	currentErr             error
	cancelNatsSubscription context.CancelFunc
}

func (ds *Datasource) SubscribeStream(ctx context.Context, request *backend.SubscribeStreamRequest) (*backend.SubscribeStreamResponse, error) {
	status := backend.SubscribeStreamStatusNotFound

	value := ds.streamResponsesSoFar.Get(request.Path)
	if value != nil {
		// found the stream, so we can subscribe to it.
		status = backend.SubscribeStreamStatusOK
	}
	return &backend.SubscribeStreamResponse{
		Status: status,
	}, nil
}

func (ds *Datasource) PublishStream(ctx context.Context, request *backend.PublishStreamRequest) (*backend.PublishStreamResponse, error) {
	// we do not allow any write operation from the frontend (so far)
	return &backend.PublishStreamResponse{
		Status: backend.PublishStreamStatusPermissionDenied,
	}, nil
}

func (ds *Datasource) RunStream(ctx context.Context, request *backend.RunStreamRequest, sender *backend.StreamSender) error {
	value := ds.streamResponsesSoFar.Get(request.Path)
	if value != nil {
		for {
			select {
			case <-ctx.Done():
				// we are done.
				value.Value().cancelNatsSubscription()
				return nil
			case <-value.Value().onNewMessages:
				// new message
				if value.Value().currentErr != nil {
					// error while processing messages -> exit stream
					return value.Value().currentErr
				}

				// no eror -> send the updated frame to the user.
				if value.Value().currentFrame != nil {
					err := sender.SendFrame(value.Value().currentFrame, data.IncludeAll)
					if err != nil {
						// TODO: close NATS subscription!
						return value.Value().currentErr
					}
				}
			}
		}
		return nil
	}
	return fmt.Errorf("no data found for stream %s", request.Path)
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

func (ds *Datasource) query(ctx context.Context, pCtx backend.PluginContext, query backend.DataQuery) backend.DataResponse {
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
	//defer natsConn.Close()

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
		frame, err := ds.requestReply(ctx, natsConn, qm)
		if err != nil {
			return backend.ErrDataResponse(backend.StatusBadRequest, "Response conversion error: "+err.Error())
		}

		return backend.DataResponse{
			Frames: data.Frames{
				frame,
			},
			Status: backend.StatusOK,
		}
	} else if qm.QueryType == QueryTypeSubscribe {
		return ds.subscribe(ctx, qm, query, pCtx, natsConn)
	} else {
		return backend.ErrDataResponse(backend.StatusBadRequest, "Invalid Query Type: "+qm.QueryType)
	}
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

func (ds *Datasource) requestReply(ctx context.Context, natsConn *nats.Conn, qm queryModel) (*data.Frame, error) {
	resp, err := natsConn.Request(qm.NatsSubject, []byte(qm.RequestData), qm.RequestTimeout.Duration)
	if err != nil {
		return nil, err
	}

	return goja.ConvertMessage(ctx, resp, qm.JsFn)
}

// subscribe handles a NATS subscription call in streaming fashin.
// TODO explain how done
// inspired by https://github.com/grafana/grafana-iot-twinmaker-app/blob/0947ce1ff0afec8372cae624566726e68687137b/pkg/plugin/datasource.go
func (ds *Datasource) subscribe(_ context.Context, qm queryModel, query backend.DataQuery, pCtx backend.PluginContext, natsConn *nats.Conn) backend.DataResponse {
	requestUuid := uuid.NewString()

	// if the context is cancelled, the NATS subscription should end.
	ctx, cancel := context.WithCancel(context.Background())
	sr := &streamResponse{
		onNewMessages:          make(chan bool, 100), // we use a buffered channel here, because we do not want to block at all, if possible.
		currentFrame:           nil,
		cancelNatsSubscription: cancel,
	}

	// TODO: do not hardcode TTL here.
	ds.streamResponsesSoFar.Set(requestUuid, sr, 5*time.Minute)

	// NOTE: we wait until the 1st
	wg := sync.WaitGroup{}
	wg.Add(1)
	i := 0
	var subscription *nats.Subscription
	var err error
	var firstFrame *data.Frame
	subscription, err = natsConn.Subscribe(qm.NatsSubject, func(msg *nats.Msg) {
		select {
		case <-ctx.Done():
			log.DefaultLogger.Debug("Cancelling NATS subscription")
			subscription.Unsubscribe()
		default:
			log.DefaultLogger.Debug("Received NATS Message")
			// extend TTL everytime we receive a msg.
			ds.streamResponsesSoFar.Touch(requestUuid)
			i++
			convertedMessage, err := goja.ConvertStreamingMessage(context.Background(), msg, qm.JsFn)
			if err != nil {
				log.DefaultLogger.Error(fmt.Sprintf("could not convert message %d - error in tamarin script: %s", i, err))

				sr.currentErr = fmt.Errorf("could not convert message %d - error in tamarin script: %w", i, err)
				sr.onNewMessages <- true
				subscription.Unsubscribe()
				if i == 1 {
					// for 1st message, answer synchronously
					wg.Done()
				}
				return
			}
			log.DefaultLogger.Debug(fmt.Sprintf("ConvMsg %v", convertedMessage))
			frame, err := framestruct.ToDataFrame("request", convertedMessage)
			// err = incrementalDataframe.AddRow(convertedMessage)
			if err != nil {
				log.DefaultLogger.Error(fmt.Sprintf("could not convert message %d - could not be converted to data frame: %s", i, err))

				sr.currentErr = fmt.Errorf("could not convert message %d - could not be converted to data frame: %w", i, err)
				sr.onNewMessages <- true
				subscription.Unsubscribe()
				if i == 1 {
					// for 1st message, answer synchronously
					wg.Done()
				}
				return
			}

			// no error :) -> notify sender
			if i == 1 {
				// for 1st message, answer synchronously
				firstFrame = frame
				wg.Done()
			} else {
				sr.currentFrame = frame
				sr.onNewMessages <- true
			}
		}
	})

	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "could not create subscription: "+err.Error())
	}

	// wait until the 1st NATS message was received
	wg.Wait()

	if sr.currentErr != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, "error handling 1st message: "+sr.currentErr.Error())
	}

	channel := live.Channel{
		Scope:     live.ScopeDatasource,
		Namespace: ds.uid,
		Path:      requestUuid, // because request UUID is random, we cannot snoop on other people's values (security). and we have one subscription per user (which is what we want in our case)
	}
	firstFrame.SetMeta(&data.FrameMeta{Channel: channel.String()})
	return backend.DataResponse{
		Frames: data.Frames{
			firstFrame,
		},
		Status: backend.StatusOK,
	}
}
