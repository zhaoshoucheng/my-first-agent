package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleYAML = `
account:
  source:
    type: file
    file:
      dir: ./config/accounts
    db:
      driver: ""
      dsn: ""
      table: ""
agent:
  max_iterations: 7
  verbose: true
  type: react
memory:
  type: buffer
  max_size: 10
tools:
  enabled:
    - calculator
    - search
  search:
    provider: google
    max_results: 5
logging:
  level: info
  format: json
`

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return p
}

func TestInit_PopulatesGetConfig(t *testing.T) {
	if err := Init(writeTempConfig(t, sampleYAML)); err != nil {
		t.Fatalf("Init: %v", err)
	}

	got := GetConfig()
	if got == nil {
		t.Fatal("GetConfig() returned nil after Init")
	}

	if got.Account.Source.Type != "file" {
		t.Errorf("Account.Source.Type = %q, want file", got.Account.Source.Type)
	}
	if got.Account.Source.File.Dir != "./config/accounts" {
		t.Errorf("Account.Source.File.Dir = %q", got.Account.Source.File.Dir)
	}
	if got.Agent.MaxIterations != 7 || !got.Agent.Verbose || got.Agent.Type != "react" {
		t.Errorf("Agent = %+v", got.Agent)
	}
	if got.Memory.Type != "buffer" || got.Memory.MaxSize != 10 {
		t.Errorf("Memory = %+v", got.Memory)
	}
	if len(got.Tools.Enabled) != 2 || got.Tools.Enabled[0] != "calculator" {
		t.Errorf("Tools.Enabled = %+v", got.Tools.Enabled)
	}
	if got.Tools.Search.Provider != "google" || got.Tools.Search.MaxResults != 5 {
		t.Errorf("Tools.Search = %+v", got.Tools.Search)
	}
	if got.Logging.Level != "info" || got.Logging.Format != "json" {
		t.Errorf("Logging = %+v", got.Logging)
	}
}

func TestSetConfig_Override(t *testing.T) {
	SetConfig(&Settings{Agent: AgentSettings{MaxIterations: 99}})
	if got := GetConfig().Agent.MaxIterations; got != 99 {
		t.Errorf("SetConfig didn't take effect, got MaxIterations=%d", got)
	}
}
