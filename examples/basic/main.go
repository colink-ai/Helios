package main

import (
	"context"
	"fmt"

	"github.com/colink-ai/helios/adapters/all"
	"github.com/colink-ai/helios/contracts"
	helios "github.com/colink-ai/helios/runtime"
)

func main() {
	engine, err := newEngine()
	if err != nil {
		panic(err)
	}
	_ = engine
}

func newEngine() (*helios.Engine, error) {
	registry := helios.NewRegistry()
	if err := all.Register(registry); err != nil {
		return nil, err
	}
	engine := helios.NewEngine(registry, helios.WithEventSink(helios.EventSinkFunc(func(_ context.Context, event contracts.RunEvent) error {
		fmt.Println(event.Type)
		return nil
	})))
	return engine, nil
}
