package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/colink-ai/helios/adapters/all"
	helios "github.com/colink-ai/helios/runtime"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("helios-compat", flag.ContinueOnError)
	flags.SetOutput(stderr)
	agentType := flags.String("agent", "hermes", "agent adapter type")
	cliPath := flags.String("cli", "", "agent CLI path")
	configMode := flags.String("runtime-config-mode", "", "runtime config mode: isolated or user")
	runtimeHome := flags.String("runtime-home", "", "agent runtime home/config directory")
	workDir := flags.String("workdir", "", "agent process working directory")
	input := flags.String("input", "Say hello from Helios.", "probe prompt")
	scenarios := flags.String("scenarios", "detect,one_shot,resident", "comma-separated scenarios")
	timeout := flags.Duration("timeout", 2*time.Minute, "per-scenario timeout")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	registry := helios.NewRegistry()
	if err := all.Register(registry); err != nil {
		return exit(stderr, err)
	}
	mode := helios.RuntimeConfigMode(*configMode)
	if mode != "" && mode != helios.RuntimeConfigIsolated && mode != helios.RuntimeConfigUser {
		return exit(stderr, fmt.Errorf("runtime-config-mode must be %q or %q", helios.RuntimeConfigIsolated, helios.RuntimeConfigUser))
	}
	engine := helios.NewEngine(registry)
	harness := helios.NewCompatibilityHarness(engine)
	report := harness.Run(context.Background(), helios.AgentSpec{
		Type:              *agentType,
		CLIPath:           *cliPath,
		RuntimeConfigMode: mode,
		RuntimeHome:       *runtimeHome,
		WorkDir:           *workDir,
	}, checks(*scenarios, *input, *timeout))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return exit(stderr, err)
	}
	fmt.Fprintln(stdout, string(data))
	for _, result := range report.Results {
		if !result.Passed {
			return 1
		}
	}
	return 0
}

func checks(value string, input string, timeout time.Duration) []helios.CompatibilityCheck {
	parts := strings.Split(value, ",")
	out := make([]helios.CompatibilityCheck, 0, len(parts))
	for _, part := range parts {
		scenario := helios.CompatibilityScenario(strings.TrimSpace(part))
		if scenario == "" {
			continue
		}
		out = append(out, helios.CompatibilityCheck{Scenario: scenario, Input: input, Timeout: timeout})
	}
	return out
}

func exit(w io.Writer, err error) int {
	fmt.Fprintln(w, err)
	return 1
}
