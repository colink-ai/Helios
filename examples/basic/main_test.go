package main

import "testing"

func TestMainFunction(t *testing.T) {
	main()
}

func TestNewEngine(t *testing.T) {
	engine, err := newEngine()
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if engine == nil {
		t.Fatalf("engine is nil")
	}
}
