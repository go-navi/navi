package watcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
	"github.com/go-navi/navi/internal/logger"
	"github.com/go-navi/navi/internal/utils"
)

// Default folders to exclude from file watching
var defaultIgnoredFolders = []string{
	"node_modules",
	"build",
	"dist",
	"out",
	"target",
	"venv",
	"env",
	"tests",
	"android",
	"ios",
	"bundle",
	"vendor",
	"tmp",
	"public",
	"site",
	"docs",
	"uploads",
	"coverage",
	"pkg",
	"bin",
	"packages",
	"Thumbs.db",
	"cdk.out",
	".git",
	".svn",
	".hg",
	".idea",
	".vscode",
	".vs",
	".gradle",
	".pytest_cache",
	"*.egg-info",
	".venv",
	".next",
	".nuxt",
	".bundle",
	".cache",
	".pnp",
	".turbo",
	".github",
	".husky",
	".jenkins",
	".docker",
	".cpcache",
	".shadow-cljs",
	".expo",
	".cargo",
	".serverless",
	".vercel",
	".terraform",
	".cache-loader",
	".astro",
	".ipynb_checkpoints",
	".settings",
	".config",
	".yarn",
	".npm",
	".nx",
	".DS_Store",
	"__pycache__",
	"__tests__",
	"__snapshots__",
	"__mocks__",
	"__generated__",
}

// FilePatterns defines include/exclude patterns for file watching
type FilePatterns struct {
	Include []string
	Exclude []string
}

// WatcherConfig stores the processed watching configuration
type WatcherConfig struct {
	DirectoriesToWatch []string
	FileInclusions     []string
	FileExclusions     []string
}

// FileWatcher monitors file system changes based on configured patterns
type FileWatcher struct {
	fsNotifier   *fsnotify.Watcher
	config       WatcherConfig
	changeNotify chan struct{}
	shutdownChan chan struct{}
}

// NewFileWatcher creates a file watcher with the specified patterns
func NewFileWatcher(filePatterns FilePatterns, logPrefixFn func() string) (*FileWatcher, error) {
	fsNotifier, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Could not enable watch mode: %w", err)
	}

	config, err := buildWatcherConfig(filePatterns, logPrefixFn)
	if err != nil {
		return nil, fmt.Errorf("Invalid format for `watch` config: %w", err)
	}

	return &FileWatcher{
		fsNotifier:   fsNotifier,
		config:       *config,
		changeNotify: make(chan struct{}, 1),
		shutdownChan: make(chan struct{}),
	}, nil
}

// Start begins watching directories and returns a channel for change notifications
func (fw *FileWatcher) Start(logPrefixFn func() string) (<-chan struct{}, error) {
	// Add all directories to the watcher
	for _, dir := range fw.config.DirectoriesToWatch {
		if err := fw.fsNotifier.Add(dir); err != nil {
			return nil, fmt.Errorf("Failed to watch directory `%s`: %w", dir, err)
		}
	}

	if len(fw.config.DirectoriesToWatch) == 0 {
		logger.WarnWithPrefix(logPrefixFn(), "WARNING: No directories found to watch for changes")
	}

	go fw.monitorEvents(logPrefixFn)

	return fw.changeNotify, nil
}

// monitorEvents processes filesystem events and notifies on relevant changes
func (fw *FileWatcher) monitorEvents(logPrefixFn func() string) {
	for {
		select {
		case event, ok := <-fw.fsNotifier.Events:
			if !ok {
				return
			}

			// Skip temporary files
			if isTempFile(event.Name) {
				continue
			}

			// Notify if path matches tracking criteria
			if shouldTrackPath(event.Name, fw.config) {
				select {
				case fw.changeNotify <- struct{}{}:
				default:
					// Channel full, skip notification
				}
			}

		case err, ok := <-fw.fsNotifier.Errors:
			if err != nil {
				logger.ErrorWithPrefix(logPrefixFn(), "File watching error: %v", err)
				return
			}

			if !ok {
				return
			}

		case <-fw.shutdownChan:
			return
		}
	}
}

// Stop terminates file watching and cleans up resources
func (fw *FileWatcher) Stop() error {
	close(fw.shutdownChan)
	return fw.fsNotifier.Close()
}

// shouldTrackPath determines if a path should trigger a watch notification
func shouldTrackPath(path string, config WatcherConfig) bool {
	// Check exclusions first
	for _, exclusion := range config.FileExclusions {
		pattern := exclusion
		// Match file directly
		if !strings.HasSuffix(pattern, "/") && matchesGlobPattern(pattern, path) {
			return false
		}

		// Match as directory content
		if matchesGlobPattern(utils.EnsureSuffix(pattern, "/*"), path) {
			return false
		}
	}

	// Then check inclusions
	for _, inclusion := range config.FileInclusions {
		pattern := inclusion
		// Match file directly
		if !strings.HasSuffix(pattern, "/") && matchesGlobPattern(pattern, path) {
			return true
		}

		// Match as directory content
		if matchesGlobPattern(utils.EnsureSuffix(pattern, "/*"), path) {
			return true
		}
	}

	return false
}

// isTempFile detects common temporary file patterns
func isTempFile(path string) bool {
	filename := filepath.Base(path)
	return (strings.HasPrefix(filename, ".") && strings.HasSuffix(filename, ".swp")) ||
		(strings.HasPrefix(filename, "~") && strings.HasSuffix(filename, ".tmp")) ||
		strings.HasSuffix(filename, ".tmp")
}

// matchesGlobPattern checks if a path matches a glob pattern
func matchesGlobPattern(pattern, name string) bool {
	matched, err := doublestar.Match(pattern, filepath.ToSlash(name))
	if err != nil || !matched {
		return false
	}

	return true
}

// buildWatcherConfig processes file patterns into a concrete watcher configuration
func buildWatcherConfig(patterns FilePatterns, logPrefixFn func() string) (*WatcherConfig, error) {
	config := &WatcherConfig{
		DirectoriesToWatch: []string{},
		FileInclusions:     patterns.Include,
		FileExclusions:     patterns.Exclude,
	}

	// Get list of folders to ignore
	foldersToIgnore := filterDefaultIgnoredFolders(patterns.Include)
	trackedDirs := make(map[string]bool)
	skippedFolderWarnings := make(map[string]bool)

	// Process include patterns to find directories to watch
	for _, pattern := range patterns.Include {
		matchedDirs, err := findDirectories(pattern, patterns.Exclude, foldersToIgnore, skippedFolderWarnings)
		if err != nil {
			return nil, err
		}

		// Add unique directories to watch list
		for _, dir := range matchedDirs {
			if !trackedDirs[dir] {
				trackedDirs[dir] = true
				config.DirectoriesToWatch = append(config.DirectoriesToWatch, dir)
			}
		}
	}

	// Prepare warning about skipped folders
	skippedFoldersList := make([]string, 0, len(skippedFolderWarnings))
	for folder := range skippedFolderWarnings {
		skippedFoldersList = append(skippedFoldersList, folder)
	}

	sort.Strings(skippedFoldersList)

	if len(skippedFoldersList) > 0 {
		logger.WarnWithPrefix(logPrefixFn(), "Skipping directories [\"%s\"] by default. Add them to `watch` parameter to monitor",
			strings.Join(skippedFoldersList, "\", \""))
	}

	return config, nil
}

// filterDefaultIgnoredFolders removes any ignored folders that are explicitly included
func filterDefaultIgnoredFolders(includePatterns []string) []string {
	filteredFolders := []string{}

	for _, folder := range defaultIgnoredFolders {
		shouldExclude := true
		folder = strings.TrimSpace(folder)

		if folder == "" {
			continue
		}

		// Check if folder is explicitly included in any pattern
		for _, pattern := range includePatterns {
			if strings.Contains(pattern, "/"+folder+"/") ||
				strings.HasSuffix(pattern, "/"+folder) ||
				strings.HasPrefix(pattern, folder+"/") ||
				pattern == folder {
				shouldExclude = false
				break
			}
		}

		if shouldExclude {
			filteredFolders = append(filteredFolders, folder)
		}
	}

	return filteredFolders
}

// findDirectories locates all directories matching the given pattern
func findDirectories(
	pattern string,
	exclusionPatterns []string,
	ignoredFolders []string,
	skippedFolderWarnings map[string]bool,
) ([]string, error) {
	matchedDirs := []string{}

	// Helper function to process a directory pattern
	processDirectoryPattern := func(dirPattern string) error {
		basePath, matchPattern := doublestar.SplitPattern(filepath.ToSlash(dirPattern))
		filesystem := os.DirFS(basePath)

		if strings.TrimSpace(matchPattern) == "" {
			matchPattern = "."
		}

		// Walk through directories matching the pattern
		err := doublestar.GlobWalk(filesystem, matchPattern, func(match string, dirEntry fs.DirEntry) error {
			if !dirEntry.IsDir() {
				return nil
			}

			matchedDir := filepath.Join(basePath, match)
			normPath := utils.EnsureSuffix(filepath.ToSlash(matchedDir), "/")

			// Skip ignored folders
			for _, folder := range ignoredFolders {
				ignoredPattern := utils.EnsureSuffix(
					filepath.ToSlash(filepath.Join(basePath, "**/"+folder+"/**/")), "/")

				if matchesGlobPattern(ignoredPattern, normPath) {
					skippedFolderWarnings[folder] = true
					return fs.SkipDir
				}
			}

			// Skip excluded patterns
			for _, exclusion := range exclusionPatterns {
				if strings.HasSuffix(exclusion, "/") && matchesGlobPattern(exclusion, normPath) {
					return fs.SkipDir
				}
			}

			matchedDirs = append(matchedDirs, matchedDir)
			return fs.SkipDir
		})

		if err != nil && err != fs.SkipDir {
			return err
		}
		return nil
	}

	// Handle file pattern (get parent directory)
	if !strings.HasSuffix(pattern, "/") {
		parentDir := utils.EnsureSuffix(
			filepath.ToSlash(filepath.Dir(pattern)), "/")

		if err := processDirectoryPattern(parentDir); err != nil {
			return nil, err
		}
	}

	// Handle directory pattern
	dirPattern := utils.EnsureSuffix(pattern, "/")
	if err := processDirectoryPattern(dirPattern); err != nil {
		return nil, err
	}

	return matchedDirs, nil
}
