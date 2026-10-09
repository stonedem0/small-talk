package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

// snapshotConfig restores the package-level config after the test.
func snapshotConfig(t *testing.T) {
	t.Helper()
	p, d, du, a, w, h := port, debugEnabled, directoryURL, appID, wsPublicURL, heartbeatEvery
	t.Cleanup(func() {
		port, debugEnabled, directoryURL, appID, wsPublicURL, heartbeatEvery = p, d, du, a, w, h
	})
}

func TestLoadConfig_Defaults(t *testing.T) {
	snapshotConfig(t)
	for _, k := range []string{"PORT", "DEBUG", "DIRECTORY_URL", "APP_ID", "WS_PUBLIC_URL", "HEARTBEAT_INTERVAL"} {
		t.Setenv(k, "")
	}
	loadConfig()

	if port != "8080" || debugEnabled {
		t.Fatalf("port=%q debug=%v, want 8080/false", port, debugEnabled)
	}
	if directoryURL != "http://localhost:8081" || wsPublicURL != "ws://localhost:8080/ws" {
		t.Fatalf("unexpected URL defaults: %q %q", directoryURL, wsPublicURL)
	}
	if appID == "" {
		t.Fatal("appID should fall back to the hostname")
	}
	if heartbeatEvery != 5*time.Second {
		t.Fatalf("heartbeatEvery=%v, want 5s", heartbeatEvery)
	}
}

// Values that only exist in a .env file must reach the config once the file is
// loaded and loadConfig runs.
func TestLoadConfig_ReadsValuesFromDotenvFile(t *testing.T) {
	snapshotConfig(t)
	keys := []string{"PORT", "DEBUG", "DIRECTORY_URL", "APP_ID", "WS_PUBLIC_URL", "HEARTBEAT_INTERVAL"}
	for _, k := range keys {
		t.Setenv(k, "") // register restore, then unset so godotenv will set them
		os.Unsetenv(k)
	}
	env := filepath.Join(t.TempDir(), "app.env")
	content := "PORT=9090\nDEBUG=true\nDIRECTORY_URL=http://dir:1234\nAPP_ID=local-app-1\n" +
		"WS_PUBLIC_URL=wss://chat.example.com/ws\nHEARTBEAT_INTERVAL=2s\n"
	if err := os.WriteFile(env, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := godotenv.Load(env); err != nil {
		t.Fatal(err)
	}
	loadConfig()

	if port != "9090" || !debugEnabled {
		t.Fatalf("port=%q debug=%v", port, debugEnabled)
	}
	if directoryURL != "http://dir:1234" || appID != "local-app-1" || wsPublicURL != "wss://chat.example.com/ws" {
		t.Fatalf("dotenv values ignored: %q %q %q", directoryURL, appID, wsPublicURL)
	}
	if heartbeatEvery != 2*time.Second {
		t.Fatalf("heartbeatEvery=%v, want 2s", heartbeatEvery)
	}
}

func TestLoadConfig_ProcessEnvBeatsDotenv(t *testing.T) {
	snapshotConfig(t)
	t.Setenv("APP_ID", "from-process")
	env := filepath.Join(t.TempDir(), "app.env")
	_ = os.WriteFile(env, []byte("APP_ID=from-file\n"), 0o600)
	_ = godotenv.Load(env) // godotenv.Load never overrides an existing variable
	loadConfig()

	if appID != "from-process" {
		t.Fatalf("appID=%q, want the process env value", appID)
	}
}
