package i18n

import (
	"testing"
)

func TestAllLanguagesLoaded(t *testing.T) {
	langs := GetLanguages()
	if len(langs) != 43 {
		t.Fatalf("expected 43 European languages, got %d", len(langs))
	}

	requiredKeys := []string{
		"menu_file", "menu_commands", "act_copy", "act_delete",
		"f1_help", "f5_copy", "col_name", "col_size", "dlg_copy_title",
	}

	for _, l := range langs {
		dict := GetDict(l.Code)
		if dict == nil {
			t.Errorf("language %s (%s) has nil dictionary", l.Code, l.Name)
			continue
		}
		for _, k := range requiredKeys {
			val, exists := dict[k]
			if !exists || val == "" {
				t.Errorf("language %s missing key %s", l.Code, k)
			}
		}
	}
}

func TestNormalizeAndDetect(t *testing.T) {
	if got := Normalize("pl_PL.UTF-8"); got != "pl" {
		t.Errorf("expected 'pl', got '%s'", got)
	}
	if got := Normalize("en-US"); got != "en" {
		t.Errorf("expected 'en', got '%s'", got)
	}
	if got := Normalize("DE"); got != "de" {
		t.Errorf("expected 'de', got '%s'", got)
	}

	if !IsValid("pl") || !IsValid("en") || !IsValid("de") || !IsValid("uk") {
		t.Errorf("expected standard languages to be valid")
	}
	if IsValid("unknown_xyz") {
		t.Errorf("expected invalid language to return false")
	}
}

func TestTFallback(t *testing.T) {
	plVal := T("pl", "menu_file")
	if plVal != "Plik" {
		t.Errorf("expected 'Plik', got '%s'", plVal)
	}

	enVal := T("en", "menu_file")
	if enVal != "File" {
		t.Errorf("expected 'File', got '%s'", enVal)
	}

	deVal := T("de", "menu_file")
	if deVal != "Datei" {
		t.Errorf("expected 'Datei', got '%s'", deVal)
	}

	fallbackVal := T("unknown_lang", "menu_file")
	if fallbackVal != "Plik" {
		t.Errorf("expected fallback 'Plik', got '%s'", fallbackVal)
	}
}
