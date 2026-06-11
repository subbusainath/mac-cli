package credentials

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	if err := Save(map[Provider]string{OpenAI: "sk-test-123"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got[OpenAI] != "sk-test-123" {
		t.Fatalf("got %q", got[OpenAI])
	}
}

func TestSaveMergesExistingKeys(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	if err := Save(map[Provider]string{OpenAI: "sk-a"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(map[Provider]string{DeepSeek: "sk-b"}); err != nil {
		t.Fatal(err)
	}
	got, _ := Load()
	if got[OpenAI] != "sk-a" || got[DeepSeek] != "sk-b" {
		t.Fatalf("merge lost keys: %v", got)
	}
}

func TestSaveFileMode0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MAC_CONFIG_DIR", dir)
	if err := Save(map[Provider]string{Anthropic: "sk-c"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "credentials.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}

func TestLookupEnvWinsOverFile(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	if err := Save(map[Provider]string{OpenAI: "from-file"}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "from-env")
	key, source, ok := Lookup(OpenAI)
	if !ok || key != "from-env" || source != "env" {
		t.Fatalf("got %q %q %v", key, source, ok)
	}
}

func TestLookupFallsBackToFile(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "") // empty env must not count
	if err := Save(map[Provider]string{OpenAI: "from-file"}); err != nil {
		t.Fatal(err)
	}
	key, source, ok := Lookup(OpenAI)
	if !ok || key != "from-file" || source != "file" {
		t.Fatalf("got %q %q %v", key, source, ok)
	}
}

func TestLookupMissing(t *testing.T) {
	t.Setenv("MAC_CONFIG_DIR", t.TempDir())
	t.Setenv("DEEPSEEK_API_KEY", "")
	if _, _, ok := Lookup(DeepSeek); ok {
		t.Fatal("want not found")
	}
}
