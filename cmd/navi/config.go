package navi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/joho/godotenv"
)

// Global configuration variables
var (
	applicationRootPath string // Root directory of the application
	configurationPath   string // Path to the configuration file
	cachedYamlFile      string // Cached yaml file string
)

// getYamlConfiguration loads and parses YAML config into the specified type
func getYamlConfiguration(replaceEnvVars bool) (YamlConfig, yaml.CommentMap, error) {
	var configResult YamlConfig

	content := string(cachedYamlFile)
	if content == "" {
		fileData, err := os.ReadFile(configurationPath)
		if err != nil {
			return configResult, nil, err
		}

		content = string(fileData)
		cachedYamlFile = string(fileData)
	}

	if replaceEnvVars {
		content = replaceEnvironmentVariables(
			replaceRootPathPlaceholders(content),
			true,
		)
	}

	commentsMap := yaml.CommentMap{}

	return configResult, commentsMap, yaml.UnmarshalWithOptions(
		[]byte(content),
		&configResult,
		yaml.Strict(),
		yaml.CommentToMap(commentsMap),
	)
}

// resolveFilePath handles path resolution with support for ROOT placeholders
func resolveFilePath(targetPath string, baseDirPath string) string {
	targetPath = strings.TrimSpace(targetPath)

	// Handle ROOT placeholder references
	if strings.HasPrefix(targetPath, "__ROOT__") {
		resolvedPath := filepath.Clean(filepath.Join(
			applicationRootPath,
			strings.TrimPrefix(targetPath, "__ROOT__"),
		))

		if strings.HasPrefix(targetPath, "__ROOT__\\") {
			return filepath.FromSlash(resolvedPath)
		}

		return resolvedPath
	}

	// Handle absolute and relative paths
	if filepath.IsAbs(targetPath) {
		return filepath.Clean(targetPath)
	}

	if targetPath != "" {
		return filepath.Clean(filepath.Join(baseDirPath, targetPath))
	}

	return baseDirPath
}

// parseDotEnvConfiguration processes dotenv config entries into structured format
func parseDotEnvConfiguration(dotEnvConfig any, baseDirPath string) DotEnvConfig {
	if dotEnvConfig == nil {
		return DotEnvConfig{}
	}

	var envFileConfig DotEnvConfig
	var entries []string

	// Convert the different possible input types to string slice
	switch value := dotEnvConfig.(type) {
	case string:
		entries = []string{value}
	case []any:
		for _, item := range value {
			if str, ok := item.(string); ok {
				entries = append(entries, str)
			}
		}
	case []string:
		entries = value
	}

	// Process each entry
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Parse path and optional keys
		parts := strings.Split(entry, "|")
		path := resolveFilePath(parts[0], baseDirPath)

		var keys []string
		if len(parts) > 1 {
			for _, key := range strings.Split(parts[1], ",") {
				keys = append(keys, strings.TrimSpace(key))
			}
		}

		envFileConfig.Files = append(envFileConfig.Files, DotEnvFile{
			Path: path,
			Keys: keys,
		})
	}

	envFileConfig.Valid = len(envFileConfig.Files) > 0
	return envFileConfig
}

// loadEnvironmentVariables loads and processes vars from .env files
func loadEnvironmentVariables(config DotEnvConfig) ([]string, error) {
	if !config.Valid {
		return nil, nil
	}

	envVarsMap := make(map[string]string)

	// Process each env file
	for _, file := range config.Files {
		fileEnv, err := godotenv.Read(file.Path)
		if err != nil {
			return nil, fmt.Errorf("Failed to load environment file `%s`: %v", file.Path, err)
		}

		// Handle specified keys or all keys
		if len(file.Keys) == 0 {
			// Load all variables
			for k, v := range fileEnv {
				envVarsMap[k] = replaceEnvironmentVariables(v, false)
			}
			continue
		}

		// Load only specified keys
		for _, key := range file.Keys {
			if val, exists := fileEnv[key]; exists {
				envVarsMap[key] = replaceEnvironmentVariables(val, false)
			} else {
				return nil, fmt.Errorf("Environment variable `%s` not found in file `%s`", key, file.Path)
			}
		}
	}

	return formatEnvironmentMap(envVarsMap), nil
}

// formatEnvironmentMap converts a map to KEY=VALUE string slice
func formatEnvironmentMap(envMap map[string]string) []string {
	if envMap == nil || len(envMap) == 0 {
		return nil
	}

	result := make([]string, 0, len(envMap))
	for k, v := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// convertYamlValueToString handles conversion of YAML values to string representation
func convertYamlValueToString(val any) string {
	switch v := val.(type) {
	case []any:
		items := make([]string, len(v))
		for i, item := range v {
			items[i] = convertYamlValueToString(item)
		}
		return "[" + strings.Join(items, ",") + "]"
	case map[string]any:
		var items []string
		for k, v := range v {
			items = append(items, fmt.Sprintf("%s:%v", k, convertYamlValueToString(v)))
		}
		return "{" + strings.Join(items, ",") + "}"
	case map[any]any:
		var items []string
		for k, v := range v {
			items = append(items, fmt.Sprintf("%v:%v", k, convertYamlValueToString(v)))
		}
		return "{" + strings.Join(items, ",") + "}"
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// replaceEnvironmentVariables expands environment variables in strings
func replaceEnvironmentVariables(input string, escapeChars bool) string {
	// Handle escaped dollar signs
	input = strings.ReplaceAll(input, "\\\\$", "\uF002")
	input = strings.ReplaceAll(input, "\\$", "\uF001")
	input = strings.ReplaceAll(input, "\uF002", "\\$")

	// Replace environment variables
	result := os.Expand(input, func(key string) string {
		if val, exists := os.LookupEnv(key); exists {
			if escapeChars {
				val = strings.ReplaceAll(val, "\\", "\\\\")
				val = strings.ReplaceAll(val, "\"", "\\\"")
			}
			return val
		}
		return "${" + key + "}"
	})

	// Restore escaped dollar signs
	return strings.ReplaceAll(result, "\uF001", "$")
}

// replaceRootPathPlaceholders substitutes __ROOT__ placeholders with actual paths
func replaceRootPathPlaceholders(input string) string {
	// Prepare escaped paths for substitution
	windowsPath := filepath.ToSlash(applicationRootPath)
	windowsPath = strings.TrimSuffix(windowsPath, "\\")
	windowsPath = strings.ReplaceAll(windowsPath, "\\", "\\\\")
	windowsPath = strings.ReplaceAll(windowsPath, "\"", "\\\"")

	stdPath := strings.ReplaceAll(applicationRootPath, "\\", "\\\\")
	stdPath = strings.ReplaceAll(stdPath, "\"", "\\\"")

	// Replace placeholders
	input = strings.ReplaceAll(input, "__ROOT__\\", windowsPath+"\\")
	return strings.ReplaceAll(input, "__ROOT__", stdPath)
}

// createContext creates a cancellable context wrapper
func createContext(parent context.Context) Ctx {
	ctx, cancel := context.WithCancel(parent)
	return Ctx{Ctx: ctx, Cancel: cancel}
}

func (ctx *Ctx) Err() error {
	return ctx.Ctx.Err()
}

func (ctx *Ctx) Done() <-chan struct{} {
	return ctx.Ctx.Done()
}

// Standard error definitions
var (
	ErrProcessTerminated = errors.New("Process terminated by 'interrupt' or 'termination' signal")
	ErrWatchModeRestart  = errors.New("Process terminated by watch mode restart")
)

// getRawProject returns a shallow copy of the project configuration as a map
func (proj *ProjectConfig) getRawProject() map[string]any {
	shallowCopy := make(map[string]any)

	if proj.Dir != "" {
		shallowCopy["dir"] = proj.Dir
	}

	if len(proj.Cmds) > 0 {
		shallowCopy["cmds"] = proj.Cmds
	}

	if proj.Pre != nil {
		shallowCopy["pre"] = proj.Pre
	}

	if proj.Post != nil {
		shallowCopy["post"] = proj.Post
	}

	if proj.After != nil {
		shallowCopy["after"] = proj.After
	}

	if proj.Dotenv != nil {
		shallowCopy["dotenv"] = proj.Dotenv
	}

	if proj.Watch != nil {
		shallowCopy["watch"] = proj.Watch
	}

	if len(proj.Env) > 0 {
		shallowCopy["env"] = proj.Env
	}

	if proj.Shell != "" {
		shallowCopy["shell"] = proj.Shell
	}

	return shallowCopy
}
