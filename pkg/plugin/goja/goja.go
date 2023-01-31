package goja

import (
	"context"
	"fmt"
	"github.com/dop251/goja"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nats-io/nats.go"
	"github.com/sandstormmedia/nats/pkg/plugin/framestruct"
	"reflect"
)

func ConvertMessage(ctx context.Context, msg *nats.Msg, jsFn string) (*data.Frame, error) {
	if jsFn == "" {
		jsFn = `
			JSON.parse(msg.Data)
		`
		// return {"foo": "bar", x: 42, y: 12}
		//
		//
		// x := json.unmarshal(msg.Data).unwrap()
		//x["a"] = 42
		//return x
	}

	vm := goja.New()
	vm.Set("msg", msg)
	resultWrapper, err := vm.RunString(jsFn)
	if err != nil {
		return nil, err
	}

	result := resultWrapper.Export()
	_, isMap := result.(map[string]interface{})
	_, isArrayOfMap := result.([]map[string]interface{})
	_, isFrame := result.(data.Frame)
	_, isFramePtr := result.(*data.Frame)

	if isFrame {
		frame := result.(data.Frame)
		return &frame, nil
	}
	if isFramePtr {
		frame := result.(*data.Frame)
		return frame, nil
	}
	if isMap {
		mapEl := result.(map[string]interface{})
		return framestruct.ToDataFrame("result", mapEl)
	}
	if isArrayOfMap {
		arrayOfMap := result.([]map[string]interface{})
		return framestruct.ToDataFrame("result", arrayOfMap)
	}

	return nil, fmt.Errorf("result of script must be map[string]interface{}, []map[string]interface{}, or data.Frame. Was: %v", reflect.TypeOf(result))
}

func ConvertStreamingMessage(ctx context.Context, msg *nats.Msg, jsFn string) (map[string]interface{}, error) {
	if jsFn == "" {
		jsFn = `
			JSON.parse(msg.Data)
		`
		// return {"foo": "bar", x: 42, y: 12}
		//
		//
		// x := json.unmarshal(msg.Data).unwrap()
		//x["a"] = 42
		//return x

	}
	vm := goja.New()
	vm.Set("msg", msg)
	resultWrapper, err := vm.RunString(jsFn)
	if err != nil {
		return nil, err
	}

	result := resultWrapper.Export()
	mapEl, isMap := result.(map[string]interface{})

	if isMap {
		return mapEl, nil
	}
	return nil, fmt.Errorf("result of streaming script must be map[string]interface{}. Was: %v", reflect.TypeOf(result))
}
