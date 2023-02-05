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
	"time"
)

// gojaPool contains a pool of pre-initialized goja Javascript Engines:
// They have a _setup function defined (see setupFn), which wraps the NATS api and smoothens
// incompatibilities between JS and Golang:
//
// - Msg.Data in NATS is a []byte, but in JS we only know about strings.
// - Timeouts are accepted as strings and auto-converted.
//
// JS variables starting with "_" are internal, and are immutable. JS Variables
// starting with "__" are request-scoped.
var gojaPool = sync.Pool{
	New: func() any {
		vm := goja.New()
		vm.Set("log", func(msg string) {
			log.DefaultLogger.Info(msg)
		})
		vm.Set("_bytesToStr", func(bytes []byte) string {
			return string(bytes)
		})
		vm.Set("_strToBytes", func(in string) []byte {
			return []byte(in)
		})
		vm.Set("_strToBytes", func(in string) []byte {
			return []byte(in)
		})
		vm.Set("_parseDuration", func(in string) (time.Duration, error) {
			return time.ParseDuration(in)
		})
		vm.Set("_nats", &natsW{
			NewInbox: nats.NewInbox,
			Context:  nats.Context,
			NewMsg:   nats.NewMsg,
		})

		vm.RunString(setupFn)
		return vm
	},
}

// natsW is a struct wrapper for static functions of the nats.* package, to be able
// to pass them to JavaScript as "nats" (because we cannot pass a *package* by reference).
type natsW struct {
	NewInbox func() string
	Context  func(ctx context.Context) nats.ContextOpt
	NewMsg   func(subject string) *nats.Msg
}

// setupFn is the JS setup function registered during construction of Goja. See gojaPool for details.
const setupFn = `
"use strict";
function _setup(_nats, _bytesToStr, _strToBytes, _parseDuration) {
    function wrapNc(__nc) {
        const nc = Object.create(__nc);
        
        nc.Publish = (subj, data) => __nc.Publish(subj, _strToBytes(data));
        nc.PublishRequest = (subj, reply, data) => __nc.PublishRequest(subj, reply, _strToBytes(data));
        nc.QueueSubscribe = (subj, queue, cb) => wrapSubscription(__nc.QueueSubscribe(subj, queue, (__msg) => cb(wrapMsg(__msg))));
        nc.QueueSubscribeSync = (subj, queue) => wrapSubscription(__nc.QueueSubscribeSync(subj, queue));
        nc.Request = (subj, data, timeout) => wrapMsg(__nc.Request(subj, _strToBytes(data), _parseDuration(timeout)));
        nc.RequestMsg = (msg, timeout) => wrapMsg(__nc.RequestMsg(msg, _parseDuration(timeout)));
        nc.RequestWithContext = (ctx, subj, data) => wrapMsg(__nc.RequestWithContext(ctx, subj, _strToBytes(data)));
        nc.Subscribe = (subj, cb) => wrapSubscription(__nc.Subscribe(subj, (__msg) => cb(wrapMsg(__msg))));
        nc.SubscribeSync = (subj) => wrapSubscription(__nc.SubscribeSync(subj));
		return nc;
    }
    
	function wrapMsg(__msg) {
        if (!__msg) {
            // keep falsy objects
            return __msg;
        }
		const msg = Object.create(__msg);
		Object.defineProperty(msg, "Data", {
			get() {
				return _bytesToStr(__msg.Data);
			},
			set(value) {
                __msg.Data = _strToBytes(value);
			}
		});
		return msg;
	}

    function wrapNats(__nats) {
		const nats = Object.create(__nats);
        
        nats.NewMsg = (subject) => wrapMsg(__nats.NewMsg(subject));
        
        return nats;
	}
    
    function nullOnTimeout(func) {
        return function() {
            try {
            	return func.apply(this, arguments);
			} catch (e) {
                return null;
			}
        }
	}
    
    function wrapSubscription(__subscription) {
        if (!__subscription) {
            // keep falsy objects
            return __subscription;
        }
        
        const subscription = Object.create(__subscription);
        subscription.NextMsg = nullOnTimeout((timeout) => wrapMsg(__subscription.NextMsg(_parseDuration(timeout))));
        subscription.NextMsgWithContext = (ctx) => wrapMsg(__subscription.NextMsgWithContext(ctx));
        
        return subscription;
    } 
    
    return function(__nc, __msg = null) {
		const nats = wrapNats(_nats);
        const nc = wrapNc(__nc);
		const msg = wrapMsg(__msg);
		return {
            nats,
            nc,
            msg
		};  
    }
    
    
}
`

// wrapJs wraps the user-defined script.
func wrapJs(in string) string {
	return fmt.Sprintf(`
	"use strict";
	(function() {
		const {nats, nc, msg} = _setup(_nats, _bytesToStr, _strToBytes, _parseDuration)(__nc, __msg);
		%s;
    })()
`, in)
}
func wrapJsScript(in string) string {
	return fmt.Sprintf(`
	"use strict";
	(function() {
		const {nats, nc} = _setup(_nats, _bytesToStr, _strToBytes, _parseDuration)(__nc, undefined);
		%s;
    })()
`, in)
}

func ConvertMessage(nc *nats.Conn, msg *nats.Msg, jsFn string) (*data.Frame, error) {
	if jsFn == "" {
		jsFn = `
			return JSON.parse(msg.Data);
		`
	}
	vm := gojaPool.Get().(*goja.Runtime)
	defer gojaPool.Put(vm)
	if err := vm.Set("__nc", nc); err != nil {
		return nil, err
	}
	if err := vm.Set("__msg", msg); err != nil {
		return nil, err
	}
	// reset request-scoped variables - this way, we can have a clean VM again.
	defer func() {
		_ = vm.GlobalObject().Delete("__nc")
		_ = vm.GlobalObject().Delete("__msg")
	}()

	resultWrapper, err := vm.RunString(wrapJs(jsFn))
	if err != nil {
		return nil, fmt.Errorf("could not run JS: %w  - JS was: %s", err, wrapJs(jsFn))
	}

	result := resultWrapper.Export()
	return convertResult(result)
}

func RunScript(nc *nats.Conn, jsFn string) (*data.Frame, error) {
	if jsFn == "" {
		return nil, fmt.Errorf("script must be specified")
	}
	vm := gojaPool.Get().(*goja.Runtime)
	defer gojaPool.Put(vm)
	if err := vm.Set("__nc", nc); err != nil {
		return nil, err
	}
	// reset request-scoped variables - this way, we can have a clean VM again.
	defer func() {
		_ = vm.GlobalObject().Delete("__nc")
	}()

	resultWrapper, err := vm.RunString(wrapJsScript(jsFn))
	if err != nil {
		return nil, fmt.Errorf("could not run JS: %w  - JS was: %s", err, wrapJs(jsFn))
	}

	result := resultWrapper.Export()
	return convertResult(result)
}

func convertResult(result interface{}) (*data.Frame, error) {
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
