package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Set both paths to test the real native installer and engine tool protocol.
// No model, API key, network, or existing agent home is required.
func TestRealEngineIntegration(t *testing.T) {
	agent := os.Getenv("SCAFFOLD_TEST_AGENT")
	binary := os.Getenv("SCAFFOLD_TEST_BINARY")
	if agent == "" || binary == "" {
		t.Skip("set SCAFFOLD_TEST_AGENT and SCAFFOLD_TEST_BINARY for real-engine integration")
	}
	agent, err := filepath.Abs(agent)
	if err != nil {
		t.Fatal(err)
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZAGENT_WORKSPACE", "")
	home := filepath.Join(t.TempDir(), "agent home")
	workspace := t.TempDir()
	invoke := func(command string, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, command, args...)
		out, err := c.CombinedOutput()
		return string(out), err
	}
	output, err := invoke(binary, "--install", home)
	if err != nil {
		t.Fatalf("installation: %v\n%s", err, output)
	}
	if output, err = invoke(binary, "--install", home); err == nil {
		t.Fatal("installer overwrote existing installation")
	}
	output, err = invoke(agent, "tools", "list", "--home", home)
	if err != nil || !strings.Contains(output, "android_project_scaffold") {
		t.Fatalf("tool discovery: %v\n%s", err, output)
	}
	request := Request{Name: `Fish & Chips "100%" $value नमस्ते`, Package: "com.example.integration", Output: "new app"}
	args, _ := json.Marshal(request)
	output, err = invoke(agent, "tools", "run", "android_project_scaffold", "--home", home, "--workspace", workspace, "--args", string(args))
	if err != nil || !strings.Contains(output, `"gradle_project_name":"integration"`) {
		t.Fatalf("engine generation: %v\n%s", err, output)
	}
	if _, err = os.Stat(filepath.Join(workspace, "new app", "gradle/wrapper/gradle-wrapper.jar")); err != nil {
		t.Fatal(err)
	}
	output, err = invoke(agent, "tools", "run", "android_project_scaffold", "--home", home, "--workspace", workspace, "--args", string(args))
	if err == nil || !strings.Contains(output, "already exists") {
		t.Fatalf("engine overwriting check: %v\n%s", err, output)
	}
	request.Output = filepath.Join(t.TempDir(), "outside")
	args, _ = json.Marshal(request)
	output, err = invoke(agent, "tools", "run", "android_project_scaffold", "--home", home, "--workspace", workspace, "--args", string(args))
	if err == nil || !strings.Contains(output, "inside ZAGENT_WORKSPACE") {
		t.Fatalf("engine containment: %v\n%s", err, output)
	}
	output, err = invoke(agent, "tools", "run", "android_project_scaffold", "--home", home, "--workspace", workspace, "--args", `{"name":"App","package":"com.example.app","output":"other","min_sdk":1}`)
	if err == nil {
		t.Fatalf("engine accepted schema violation: %s", output)
	}
}
