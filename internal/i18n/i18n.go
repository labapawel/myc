package i18n

import (
	_ "embed"
	"encoding/json"
	"os"
	"strings"
	"sync"
)

//go:embed translations.json
var translationsData []byte

type Language struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	English string `json:"english"`
	Flag    string `json:"flag"`
}

type i18nContainer struct {
	Languages    []Language                   `json:"languages"`
	Translations map[string]map[string]string `json:"translations"`
}

var (
	instance *i18nContainer
	once     sync.Once
)

func getContainer() *i18nContainer {
	once.Do(func() {
		instance = &i18nContainer{}
		if err := json.Unmarshal(translationsData, instance); err != nil {
			instance.Languages = []Language{
				{Code: "pl", Name: "Polski", English: "Polish", Flag: "🇵🇱"},
				{Code: "en", Name: "English", English: "English", Flag: "🇬🇧"},
			}
			instance.Translations = make(map[string]map[string]string)
		}
	})
	return instance
}

// GetLanguages returns all supported European languages.
func GetLanguages() []Language {
	return getContainer().Languages
}

// RawJSON returns raw translations JSON for web frontend.
func RawJSON() []byte {
	return translationsData
}

// IsValid checks if language code is supported.
func IsValid(code string) bool {
	code = Normalize(code)
	c := getContainer()
	for _, l := range c.Languages {
		if l.Code == code {
			return true
		}
	}
	return false
}

// Normalize normalizes language code (e.g. "pl_PL.UTF-8" -> "pl", "EN-US" -> "en").
func Normalize(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if idx := strings.IndexAny(raw, "._-@"); idx != -1 {
		raw = raw[:idx]
	}
	return raw
}

// DetectLanguage detects user language from environment variables or default.
func DetectLanguage() string {
	for _, env := range []string{"MYC_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(env)
		if v != "" {
			norm := Normalize(v)
			if IsValid(norm) {
				return norm
			}
		}
	}
	return "pl" // Default to Polish as requested
}

// T returns the translated string for a given language and key.
func T(lang, key string) string {
	c := getContainer()
	lang = Normalize(lang)

	// Try target language
	if dict, ok := c.Translations[lang]; ok {
		if val, exists := dict[key]; exists && val != "" {
			return val
		}
	}

	// Fallback to Polish
	if dict, ok := c.Translations["pl"]; ok {
		if val, exists := dict[key]; exists && val != "" {
			return val
		}
	}

	// Fallback to English
	if dict, ok := c.Translations["en"]; ok {
		if val, exists := dict[key]; exists && val != "" {
			return val
		}
	}

	return key
}

// GetDict returns the map of all translations for a given language.
func GetDict(lang string) map[string]string {
	c := getContainer()
	lang = Normalize(lang)
	if dict, ok := c.Translations[lang]; ok {
		return dict
	}
	if dict, ok := c.Translations["pl"]; ok {
		return dict
	}
	return c.Translations["en"]
}
