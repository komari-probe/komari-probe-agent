package config

import (
	"fmt"
	"reflect"
	"strconv"
)

var legacyEnvironmentNames = map[string]string{
	"AGENT_ENDPOINT": "KOMARI_ENDPOINT",
	"AGENT_TOKEN":    "KOMARI_TOKEN",
}

// ApplyEnvironment overlays values from environment variables declared in the
// env tag of Config fields. A set but invalid value is an error rather than
// being silently ignored.
func ApplyEnvironment(dst *Config, lookup func(string) (string, bool)) error {
	value := reflect.ValueOf(dst).Elem()
	typ := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		envName := typ.Field(i).Tag.Get("env")
		if envName == "" {
			continue
		}
		envValue, ok := lookup(envName)
		if !ok {
			if legacyName, exists := legacyEnvironmentNames[envName]; exists {
				envValue, ok = lookup(legacyName)
			}
		}
		if !ok {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(envValue)
		case reflect.Bool:
			parsed, err := strconv.ParseBool(envValue)
			if err != nil {
				return fmt.Errorf("parse %s as bool: %w", envName, err)
			}
			field.SetBool(parsed)
		case reflect.Int:
			parsed, err := strconv.Atoi(envValue)
			if err != nil {
				return fmt.Errorf("parse %s as integer: %w", envName, err)
			}
			field.SetInt(int64(parsed))
		case reflect.Float64:
			parsed, err := strconv.ParseFloat(envValue, 64)
			if err != nil {
				return fmt.Errorf("parse %s as number: %w", envName, err)
			}
			field.SetFloat(parsed)
		default:
			return fmt.Errorf("unsupported config field type %s for %s", field.Kind(), envName)
		}
	}
	return nil
}
