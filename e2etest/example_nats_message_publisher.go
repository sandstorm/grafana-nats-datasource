package main

import (
	"github.com/nats-io/nats.go"
	"sync"
)

func main() {
	natsConn, err := nats.Connect("127.0.0.1:4222")
	if err != nil {
		panic(err)
	}
	println("Subscribing")

	_, _ = natsConn.Subscribe("test.simpleJsonString", func(m *nats.Msg) {
		m.Respond([]byte(`"My string here"`))
	})

	_, _ = natsConn.Subscribe("test.simpleJsonObj", func(m *nats.Msg) {
		m.Respond([]byte(`[{"key":"value1","key2":"value2"}]`))
	})
	_, _ = natsConn.Subscribe("test.simpleJsonObjArr", func(m *nats.Msg) {
		m.Respond([]byte(`[{"key":"value1","key2":"value2"}, {"key":"value1a","key2":"value2a", "key3": 0}]`))
	})

	println("Subscribed")
	// keep WaitGroup open until process is killed
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()
}
