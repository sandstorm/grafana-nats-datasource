package goja

import (
	"context"
	"fmt"
	"github.com/dop251/goja"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nats-io/nats.go"
	"github.com/sandstormmedia/nats/pkg/plugin/framestruct"
	"reflect"
	"sync"
)

var gojaPool = sync.Pool{
	New: func() any {
		goja := goja.New()
		return goja
	},
}

func wrapJs(in string) string {
	return fmt.Sprintf(`
"use strict";

const msg = Object.create(__rawMsg);
Object.defineProperty(msg, "Data", {
	get() {
		return __bytesToStr(__rawMsg.Data);
	},
});




(function() {
	%s;
})()
`, in)
}
func ConvertMessage(ctx context.Context, msg *nats.Msg, jsFn string) (*data.Frame, error) {
	if jsFn == "" {
		jsFn = `
			log(msg.Data);
			return JSON.parse(msg.Data);
			return JSON.parse('{"k1": "v1"}')
		`
		// return {"foo": "bar", x: 42, y: 12}
		//
		//
		// x := json.unmarshal(msg.Data).unwrap()
		//x["a"] = 42
		//return x
	}

	vm := goja.New()
	vm.Set("log", func(msg string) {
		log.DefaultLogger.Info(msg)
	})
	vm.Set("__rawMsg", msg)
	vm.Set("__bytesToStr", func(bytes []byte) string {
		return string(bytes)
	})

	resultWrapper, err := vm.RunString(wrapJs(jsFn))
	if err != nil {
		return nil, fmt.Errorf("could not run JS: %w  - JS was: %s", err, wrapJs(jsFn))
	}

	result := resultWrapper.Export()
	_, isMap := result.(map[string]interface{})
	_, isArray := result.([]interface{})
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
	if isArray {
		arr := result.([]interface{})
		arrayOfMap := make([]map[string]interface{}, 0, len(arr))
		for i, v := range arr {
			conv, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("result of script was []any, but not []map[string]any. Index %d was of type %s", i, reflect.TypeOf(v).String())
			}
			arrayOfMap = append(arrayOfMap, conv)
		}
		return framestruct.ToDataFrame("result", arrayOfMap)
	}

	return nil, fmt.Errorf("result of script must be map[string]interface{}, []map[string]interface{}, or data.Frame. Was: %v", reflect.TypeOf(result))
}

func ConvertStreamingMessage(ctx context.Context, msg *nats.Msg, jsFn string) (map[string]interface{}, error) {
	if jsFn == "" {
		jsFn = `
			return JSON.parse(msg.Data)
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
	resultWrapper, err := vm.RunString(wrapJs(jsFn))
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
