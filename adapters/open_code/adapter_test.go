package open_code

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	helios "github.com/colink-ai/helios/runtime"
)

func TestBuildConfigContent(t *testing.T) {
	content := buildConfigContent(helios.AgentSpec{
		DefaultModel: "qwen-plus",
		APIURL:       "https://model.test/v1",
		APIToken:     "secret",
	}, "")
	if content == "" {
		t.Fatalf("content is empty")
	}
	var cfg openCodeConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.Model != "helios/qwen-plus" {
		t.Fatalf("model = %q", cfg.Model)
	}
	provider := cfg.Provider[providerID]
	if provider.Options.APIKey != "secret" || provider.Options.BaseURL != "https://model.test/v1" {
		t.Fatalf("unexpected provider: %+v", provider)
	}
	if !provider.Models["qwen-plus"].Attachment {
		t.Fatalf("model should support attachments")
	}
	if cfg.Permission != nil {
		t.Fatalf("permission should not default to allow: %+v", cfg.Permission)
	}
}

func TestBuildConfigContentEmptyAndAPIOnly(t *testing.T) {
	if got := buildConfigContent(helios.AgentSpec{}, ""); got != "" {
		t.Fatalf("empty spec content = %q", got)
	}
	content := buildConfigContent(helios.AgentSpec{APIURL: "https://model.test/v1"}, "")
	if content == "" || !strings.Contains(content, "https://model.test/v1") {
		t.Fatalf("unexpected API-only content: %s", content)
	}
}

func TestBuildConfigContentPermissionMode(t *testing.T) {
	content := buildConfigContent(helios.AgentSpec{DefaultModel: "qwen-plus"}, "allow")
	var cfg openCodeConfig
	if err := json.Unmarshal([]byte(content), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.Permission != "allow" {
		t.Fatalf("permission = %#v", cfg.Permission)
	}
}

func TestBuildEnv(t *testing.T) {
	workDir := t.TempDir()
	env := strings.Join(buildEnv(helios.SessionRequest{
		WorkDir: workDir,
		Agent:   helios.AgentSpec{DefaultModel: "qwen-plus"},
	}, config{}), "\n")
	for _, want := range []string{"OPENCODE_PURE=1", "OPENCODE_ENABLE_QUESTION_TOOL=1", "OPENCODE_CONFIG_DIR=", "OPENCODE_CONFIG_CONTENT="} {
		if !strings.Contains(env, want) {
			t.Fatalf("env missing %q: %s", want, env)
		}
	}
}

func TestConfigDirPrecedence(t *testing.T) {
	if got := configDir(helios.SessionRequest{ConfigDir: "/role/opencode", RuntimeHome: "/runtime"}); got != "/role/opencode" {
		t.Fatalf("host config dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{Agent: helios.AgentSpec{ConfigDir: "/agent/opencode"}, RuntimeHome: "/runtime"}); got != "/agent/opencode" {
		t.Fatalf("agent config dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{RuntimeHome: "/runtime", Agent: helios.AgentSpec{RuntimeHome: "/agent"}}); got != "/runtime/opencode" {
		t.Fatalf("runtime home dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{Agent: helios.AgentSpec{RuntimeHome: "/agent"}}); got != "/agent/opencode" {
		t.Fatalf("agent runtime home dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{WorkDir: "/work"}); got != "/work/.opencode" {
		t.Fatalf("work dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{Agent: helios.AgentSpec{WorkDir: "/agent-work"}}); got != "/agent-work/.opencode" {
		t.Fatalf("agent work dir = %q", got)
	}
	if got := configDir(helios.SessionRequest{RuntimeConfigMode: helios.RuntimeConfigUser, WorkDir: "/work"}); got != "" {
		t.Fatalf("user config mode should not set config dir, got %q", got)
	}
	if got := configDir(helios.SessionRequest{}); got != "" {
		t.Fatalf("empty dir = %q", got)
	}
}

func TestOptionsAndNewAdapter(t *testing.T) {
	cfg := config{}
	WithCLIPath("custom")(&cfg)
	WithHTTPPort(7777)(&cfg)
	WithPermissionMode("allow")(&cfg)
	if cfg.cliPath != "custom" || cfg.port != 7777 || cfg.permissionMode != "allow" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	adapter, ok := NewAdapter(WithCLIPath("custom"), WithHTTPPort(7777)).(*Adapter)
	if !ok || adapter == nil {
		t.Fatalf("adapter is nil")
	}
	if adapter.cfg.cliPath != "custom" || adapter.cfg.port != 7777 {
		t.Fatalf("unexpected adapter config: %+v", adapter.cfg)
	}
}

func TestMetadataOptions(t *testing.T) {
	meta := map[string]any{"httpPort": "9191", "permission": "ask"}
	if value, ok := metadataInt(meta, "httpPort"); !ok || value != 9191 {
		t.Fatalf("unexpected httpPort: %d %v", value, ok)
	}
	if value, ok := metadataString(meta, "permission"); !ok || value != "ask" {
		t.Fatalf("unexpected permission: %q %v", value, ok)
	}
}

func TestRegister(t *testing.T) {
	reg := helios.NewRegistry()
	if err := Register(reg); err != nil {
		t.Fatalf("register: %v", err)
	}
	adapter, err := reg.Create(helios.AgentSpec{Type: Type})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if adapter == nil {
		t.Fatalf("adapter is nil")
	}
}

func TestRegisterSpecCLIOverride(t *testing.T) {
	reg := helios.NewRegistry()
	if err := Register(reg, WithCLIPath("default")); err != nil {
		t.Fatalf("register: %v", err)
	}
	adapter, err := reg.Create(helios.AgentSpec{Type: Type, CLIPath: "from-spec"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if adapter == nil {
		t.Fatalf("adapter is nil")
	}
}

func TestBuildArgsDynamicPortAndSessionTracking(t *testing.T) {
	adapter := NewAdapter().(*Adapter)
	workDir := t.TempDir()
	args := adapter.buildArgs(helios.SessionRequest{SessionID: "session-1", WorkDir: workDir})
	if len(args) != 3 || args[0] != "acp" || args[1] != "--port" {
		t.Fatalf("unexpected args: %#v", args)
	}
	port, err := strconv.Atoi(args[2])
	if err != nil || port <= 0 {
		t.Fatalf("invalid port %q: %v", args[2], err)
	}
	info, ok := adapter.session("session-1")
	if !ok {
		t.Fatalf("session HTTP info not tracked")
	}
	if info.port != port || info.cwd != workDir {
		t.Fatalf("unexpected session HTTP info: %+v", info)
	}
}

func TestBuildArgsStaticPort(t *testing.T) {
	adapter := NewAdapter(WithHTTPPort(7777)).(*Adapter)
	args := adapter.buildArgs(helios.SessionRequest{SessionID: "session-1", WorkDir: t.TempDir()})
	if strings.Join(args, " ") != "acp --port 7777" {
		t.Fatalf("unexpected args: %#v", args)
	}
	info, ok := adapter.session("session-1")
	if !ok || info.port != 7777 {
		t.Fatalf("unexpected session HTTP info: %+v %v", info, ok)
	}
}

func TestOpenCodeReplyAnswers(t *testing.T) {
	answers, err := openCodeReplyAnswers(`{"question_0":"a","question_1":["b","c"]}`)
	if err != nil {
		t.Fatalf("answers: %v", err)
	}
	if len(answers) != 2 || strings.Join(answers[0], ",") != "a" || strings.Join(answers[1], ",") != "b,c" {
		t.Fatalf("unexpected answers: %#v", answers)
	}
	if _, err := openCodeReplyAnswers(`{"other":"a"}`); err == nil {
		t.Fatalf("expected missing answers error")
	}
	if _, err := openCodeReplyAnswers(`{"question_0":123}`); err == nil {
		t.Fatalf("expected unsupported answer type error")
	}
}

func TestOpenCodeQuestionReplyHTTP(t *testing.T) {
	cwd := t.TempDir()
	var posted map[string][][]string
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("directory") != cwd {
			t.Fatalf("directory query = %q", r.URL.Query().Get("directory"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/question":
			_, _ = w.Write([]byte(`[{"id":"req-1","tool":{"callID":"tool-1"}}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/question/req-1/reply":
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode reply: %v", err)
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server.Listener = listener
	server.Start()
	defer server.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	err = openCodeQuestionReply(context.Background(), cwd, "tool-1", `{"question_0":"a","question_1":["b","c"]}`, port)
	if err != nil {
		t.Fatalf("reply: %v", err)
	}
	answers := posted["answers"]
	if len(answers) != 2 || strings.Join(answers[0], ",") != "a" || strings.Join(answers[1], ",") != "b,c" {
		t.Fatalf("unexpected posted answers: %#v", posted)
	}
}

func TestOpenCodeQuestionReplyHTTPMissingTool(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"req-1","tool":{"callID":"other"}}]`))
	}))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server.Listener = listener
	server.Start()
	defer server.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	err = openCodeQuestionReply(context.Background(), t.TempDir(), "tool-1", `{"question_0":"a"}`, port)
	if err == nil || !strings.Contains(err.Error(), "no pending question") {
		t.Fatalf("expected missing question error, got %v", err)
	}
}
