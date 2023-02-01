package goja

import (
	"github.com/dop251/goja"
	"github.com/nats-io/nats.go"
	"testing"
)

func TestJsonDecode(t *testing.T) {
	msg := &nats.Msg{Data: []byte(`{"k1":"v1"}`)}
	vm := goja.New()
	//vm.GlobalObject().Set("in", in)
	vm.Set("rawMsg", msg)
	vm.Set("__bytesToStr", func(bytes []byte) string {
		return string(bytes)
	})
	val, err := vm.RunString(`
"use strict";

const msg = Object.create(rawMsg);
Object.defineProperty(msg, "Data", {
	get() {
		return __bytesToStr(rawMsg.Data);
	},
});


(function() {
	return JSON.parse(msg.Data);
})()
`)
	if err != nil {
		t.Fatalf("Goja error: %s", err)
	}
	valConv := val.Export()
	if valConv.(map[string]any)["k1"] != "v1" {
		t.Fatalf("JSON could not be parsed")
	}
}
