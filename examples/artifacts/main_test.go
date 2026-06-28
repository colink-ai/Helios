package main

import (
	"os"
	"testing"
)

func TestMainFunction(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(old)
	main()
}
