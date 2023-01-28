package tamarin

import (
	"context"
	"fmt"
	"github.com/cloudcmds/tamarin/exec"
	"github.com/cloudcmds/tamarin/object"
	"github.com/cloudcmds/tamarin/scope"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/nats-io/nats.go"
	"github.com/sandstormmedia/nats/pkg/plugin/framestruct"
	"reflect"
)

func ConvertMessage(ctx context.Context, msg *nats.Msg, tamarinFn string) (*data.Frame, error) {
	if tamarinFn == "" {
		tamarinFn = `
			json.unmarshal(msg.Data)
		`
	}
	registry, err := object.NewTypeRegistry(object.TypeRegistryOpts{
		Converters: []object.TypeConverter{
			bytesConverter{},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not create type registry: %w", err)
	}

	msgProxy, err := object.NewProxy(registry, msg)
	if err != nil {
		return nil, fmt.Errorf("could not create proxy for msg: %w", err)
	}
	s := scope.New(scope.Opts{})
	s.Declare("msg", msgProxy, true)

	result, err := exec.Execute(ctx, exec.Opts{
		Input: tamarinFn,
		Scope: s,
	})
	if err != nil {
		return nil, fmt.Errorf("execution error: %w", err)
	}

	fmt.Printf("script result: %s (type %v)\n",
		result.Inspect(), reflect.TypeOf(result))

	_, isMap := result.Interface().(map[string]interface{})
	_, isArrayOfMap := result.Interface().([]map[string]interface{})
	_, isFrame := result.Interface().(data.Frame)
	_, isFramePtr := result.Interface().(*data.Frame)

	if isFrame {
		frame := result.Interface().(data.Frame)
		return &frame, nil
	}
	if isFramePtr {
		frame := result.Interface().(*data.Frame)
		return frame, nil
	}
	if isMap {
		mapEl := result.Interface().(map[string]interface{})
		return framestruct.ToDataFrame("result", mapEl)
	}
	if isArrayOfMap {
		arrayOfMap := result.Interface().([]map[string]interface{})
		return framestruct.ToDataFrame("result", arrayOfMap)
	}

	return nil, fmt.Errorf("result of script must be map[string]interface{}, []map[string]interface{}, or data.Frame. Was: %v %s", reflect.TypeOf(result.Interface()), result.Inspect())
}

func ensureValue(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v
}
