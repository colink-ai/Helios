package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	helios "github.com/colink-ai/helios/runtime"
)

func TestChecks(t *testing.T) {
	got := checks("detect, one_shot,, resident ", "hi", 3*time.Second)
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Scenario != helios.CompatDetect || got[1].Scenario != helios.CompatOneShot || got[2].Scenario != helios.CompatResident {
		t.Fatalf("unexpected checks: %+v", got)
	}
	if got[0].Input != "hi" || got[0].Timeout != 3*time.Second {
		t.Fatalf("unexpected check data: %+v", got[0])
	}
}

func TestRunParseError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"-bad"}, &out, &errOut); code != 2 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestRunRejectsInvalidRuntimeConfigMode(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"-runtime-config-mode", "shared"}, &out, &errOut); code != 1 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), "runtime-config-mode") {
		t.Fatalf("unexpected error: %q", errOut.String())
	}
}

func TestRunFailedProbe(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"-agent", "missing", "-scenarios", "detect", "-timeout", "1ms"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"passed": false`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestExit(t *testing.T) {
	var errOut bytes.Buffer
	if code := exit(&errOut, errTest{}); code != 1 || !strings.Contains(errOut.String(), "boom") {
		t.Fatalf("unexpected exit: code=%d err=%q", code, errOut.String())
	}
}

type errTest struct{}

func (errTest) Error() string { return "boom" }
