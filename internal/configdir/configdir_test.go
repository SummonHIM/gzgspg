package configdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirUsesEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GZGSPG_CONFIG_DIR", tmp)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	if dir != tmp {
		t.Fatalf("expected %q, got %q", tmp, dir)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("dir must exist: err=%v", err)
	}
}

func TestDirDefaultsUnderUserConfigDir(t *testing.T) {
	t.Setenv("GZGSPG_CONFIG_DIR", "")

	dir, err := Dir()
	if err != nil {
		t.Fatalf("dir: %v", err)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		t.Skip("UserConfigDir unavailable")
	}
	want := filepath.Join(base, "gzgspg")
	if dir != want {
		t.Fatalf("expected %q, got %q", want, dir)
	}
}

func TestConfigAndLogPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GZGSPG_CONFIG_DIR", tmp)

	cp, err := ConfigPath()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if filepath.Base(cp) != "config.json" {
		t.Fatalf("unexpected config path: %q", cp)
	}
	if filepath.Dir(cp) != tmp {
		t.Fatalf("config path not in override dir: %q", cp)
	}

	lp, err := LogPath()
	if err != nil {
		t.Fatalf("log path: %v", err)
	}
	if !strings.HasSuffix(lp, "gzgspg.log") {
		t.Fatalf("unexpected log path: %q", lp)
	}
	if filepath.Dir(lp) != tmp {
		t.Fatalf("log path not in override dir: %q", lp)
	}
}
