package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/colink-ai/helios/adapters/all"
	helios "github.com/colink-ai/helios/runtime"
)

func main() {
	agentType := flag.String("agent", "hermes", "agent adapter type")
	cliPath := flag.String("cli", "", "agent CLI path")
	input := flag.String("input", "Say hello from Helios.", "probe prompt")
	scenarios := flag.String("scenarios", "detect,one_shot,resident", "comma-separated scenarios")
	timeout := flag.Duration("timeout", 2*time.Minute, "per-scenario timeout")
	flag.Parse()

	registry := helios.NewRegistry()
	if err := all.Register(registry); err != nil {
		exit(err)
	}
	engine := helios.NewEngine(registry)
	harness := helios.NewCompatibilityHarness(engine)
	report := harness.Run(context.Background(), helios.AgentSpec{
		Type:    *agentType,
		CLIPath: *cliPath,
	}, checks(*scenarios, *input, *timeout))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		exit(err)
	}
	fmt.Println(string(data))
	for _, result := range report.Results {
		if !result.Passed {
			os.Exit(1)
		}
	}
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

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
