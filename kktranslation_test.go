package kktranslation

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v2"
)

type testLangFile struct {
	Version string            `yaml:"version"`
	Lang    string            `yaml:"lang"`
	Name    string            `yaml:"name"`
	Dict    map[string]string `yaml:"dict"`
}

func writeLangYAML(t *testing.T, rootPath string, lf testLangFile) {
	t.Helper()

	b, err := yaml.Marshal(lf)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}

	p := filepath.Join(rootPath, lf.Lang+".yaml")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s): %v", p, err)
	}
}

func Test_IsDebug_Priority(t *testing.T) {
	t.Setenv("APP_DEBUG", "TRUE")
	t.Setenv("KKAPP_DEBUG", "FALSE")
	if !_IsDebug() {
		t.Fatalf("expected debug enabled when APP_DEBUG=TRUE")
	}

	t.Setenv("APP_DEBUG", "")
	t.Setenv("KKAPP_DEBUG", "TRUE")
	if !_IsDebug() {
		t.Fatalf("expected debug enabled when APP_DEBUG empty and KKAPP_DEBUG=TRUE")
	}

	t.Setenv("APP_DEBUG", "false")
	t.Setenv("KKAPP_DEBUG", "true")
	if _IsDebug() {
		t.Fatalf("expected debug disabled when APP_DEBUG=false even if KKAPP_DEBUG=true")
	}
}

func Test_Instance_TranslateAndFallback(t *testing.T) {
	root := t.TempDir()
	writeLangYAML(t, root, testLangFile{
		Version: "1",
		Lang:    "zh-tw",
		Name:    "Traditional Chinese",
		Dict: map[string]string{
			"hello":   "HELLO_ZH",
			"missing": "DEFAULT_ZH",
		},
	})
	writeLangYAML(t, root, testLangFile{
		Version: "1",
		Lang:    "en",
		Name:    "English",
		Dict: map[string]string{
			"hello": "Hello",
		},
	})

	t.Setenv("APP_DEBUG", "")
	t.Setenv("KKAPP_DEBUG", "")

	tr := NewWith(root, true, "zh-tw")
	if got := tr.GetLangFile("en").T("hello"); got != "Hello" {
		t.Fatalf("expected en hello=Hello, got %q", got)
	}
	if got := tr.GetLangFile("en").T("missing"); got != "DEFAULT_ZH" {
		t.Fatalf("expected fallback to default language, got %q", got)
	}

	trNoFallback := NewWith(root, false, "zh-tw")
	if got := trNoFallback.GetLangFile("en").T("missing"); got != "missing" {
		t.Fatalf("expected no fallback when translateFallback=false, got %q", got)
	}
}

func Test_PackageLevelWrapper_UsesGlobals(t *testing.T) {
	root := t.TempDir()
	writeLangYAML(t, root, testLangFile{
		Version: "1",
		Lang:    "zh-tw",
		Name:    "Traditional Chinese",
		Dict: map[string]string{
			"hello": "HELLO_ZH",
		},
	})
	writeLangYAML(t, root, testLangFile{
		Version: "1",
		Lang:    "en",
		Name:    "English",
		Dict: map[string]string{
			"hello": "Hello",
		},
	})

	oldRoot := LangRootPath
	oldDefault := DefaultLang
	oldFallback := TranslateFallback
	t.Cleanup(func() {
		LangRootPath = oldRoot
		DefaultLang = oldDefault
		TranslateFallback = oldFallback
	})

	LangRootPath = root
	DefaultLang = "zh-tw"
	TranslateFallback = true

	// Force debug mode to avoid any cross-test cache influence on the default instance.
	t.Setenv("APP_DEBUG", "TRUE")
	t.Setenv("KKAPP_DEBUG", "")

	if got := GetLangFile("en").T("hello"); got != "Hello" {
		t.Fatalf("expected wrapper en hello=Hello, got %q", got)
	}
	if got := GetLangFile("zh-tw").T("hello"); got != "HELLO_ZH" {
		t.Fatalf("expected wrapper zh-tw hello=HELLO_ZH, got %q", got)
	}

	files := LangFiles()
	if len(files) == 0 {
		t.Fatalf("expected LangFiles() returns non-empty")
	}
}

func Test_DebugReload_ClearsBaseLangCache(t *testing.T) {
	root := t.TempDir()

	// Only create en.yaml (no en-us.yaml) so en-us resolves via base language fallback.
	writeLangYAML(t, root, testLangFile{
		Version: "1",
		Lang:    "en",
		Name:    "English",
		Dict: map[string]string{
			"hello": "v1",
		},
	})

	tr := NewWith(root, true, "en")
	t.Setenv("APP_DEBUG", "TRUE")
	t.Setenv("KKAPP_DEBUG", "")

	if got := tr.GetLangFile("en-us").T("hello"); got != "v1" {
		t.Fatalf("expected v1, got %q", got)
	}

	writeLangYAML(t, root, testLangFile{
		Version: "1",
		Lang:    "en",
		Name:    "English",
		Dict: map[string]string{
			"hello": "v2",
		},
	})

	if got := tr.GetLangFile("en-us").T("hello"); got != "v2" {
		t.Fatalf("expected v2 after modifying base language file in debug mode, got %q", got)
	}
}

func Test_Instance_DynamicFallbackProvider(t *testing.T) {
	root := t.TempDir()
	writeLangYAML(t, root, testLangFile{
		Version: "1",
		Lang:    "zh-tw",
		Name:    "Traditional Chinese",
		Dict: map[string]string{
			"missing": "DEFAULT_ZH",
		},
	})
	writeLangYAML(t, root, testLangFile{
		Version: "1",
		Lang:    "en",
		Name:    "English",
		Dict:    map[string]string{},
	})

	fallbackEnabled := true
	tr := NewWithProviders(
		func() string { return root },
		func() bool { return fallbackEnabled },
		func() string { return "zh-tw" },
		func() bool { return false },
	)

	if got := tr.GetLangFile("en").T("missing"); got != "DEFAULT_ZH" {
		t.Fatalf("expected fallback enabled, got %q", got)
	}

	fallbackEnabled = false
	if got := tr.GetLangFile("en").T("missing"); got != "missing" {
		t.Fatalf("expected fallback disabled dynamically, got %q", got)
	}
}
