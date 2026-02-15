package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveSource resolves a source string to a local directory path.
// For local paths, it canonicalizes. For git URLs, it clones/caches.
func ResolveSource(source, tag, subDir string, refresh bool) (string, error) {
	if isGitURL(source) {
		return resolveGitSource(source, tag, refresh, subDir)
	}
	return resolveLocalSource(source, subDir)
}

// isGitURL returns true if the source looks like a git URL.
func isGitURL(source string) bool {
	return strings.HasPrefix(source, "git@") ||
		strings.HasPrefix(source, "https://github.com") ||
		strings.HasPrefix(source, "https://gitlab.com") ||
		strings.HasPrefix(source, "https://bitbucket.org") ||
		strings.HasSuffix(source, ".git")
}

// resolveLocalSource canonicalizes a local path and optionally appends a subdir.
func resolveLocalSource(source, subDir string) (string, error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("resolve local source: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("source not found: %s", abs)
	}
	if subDir != "" {
		abs = filepath.Join(abs, subDir)
	}
	return abs, nil
}

// resolveGitSource clones or updates a git repo and returns the local path.
// Repos are cached under ~/.kael/cache/<provider>/<org>/<repo>.
func resolveGitSource(url, tag string, refresh bool, subDir string) (string, error) {
	cachePath, err := gitCachePath(url)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(filepath.Join(cachePath, ".git")); err == nil {
		// Already cloned
		if refresh {
			if err := gitFetch(cachePath); err != nil {
				return "", fmt.Errorf("git fetch: %w", err)
			}
			branch := gitDefaultBranch(cachePath)
			if err := gitResetHard(cachePath, "origin/"+branch); err != nil {
				return "", fmt.Errorf("git reset: %w", err)
			}
		}
	} else {
		// Fresh clone
		if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
			return "", fmt.Errorf("create cache dir: %w", err)
		}
		if err := gitClone(url, cachePath); err != nil {
			return "", fmt.Errorf("git clone: %w", err)
		}
	}

	// Checkout tag/branch if specified
	if tag != "" && tag != "latest" {
		if err := gitCheckout(cachePath, tag); err != nil {
			return "", fmt.Errorf("git checkout %s: %w", tag, err)
		}
	}

	result := cachePath
	if subDir != "" {
		result = filepath.Join(result, subDir)
	}
	return result, nil
}

// gitCachePath computes the cache directory for a git URL.
// git@github.com:org/repo.git → ~/.kael/cache/github.com/org/repo
// https://github.com/org/repo  → ~/.kael/cache/github.com/org/repo
func gitCachePath(url string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	provider, orgRepo := parseGitURL(url)
	if provider == "" || orgRepo == "" {
		return "", fmt.Errorf("cannot parse git URL: %s", url)
	}

	// Strip .git suffix
	orgRepo = strings.TrimSuffix(orgRepo, ".git")

	return filepath.Join(home, ".kael", "cache", provider, orgRepo), nil
}

// parseGitURL extracts provider and org/repo from a git URL.
func parseGitURL(url string) (provider, orgRepo string) {
	// SSH: git@github.com:org/repo.git
	if strings.HasPrefix(url, "git@") {
		parts := strings.SplitN(url[4:], ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "", ""
	}

	// HTTPS: https://github.com/org/repo
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	idx := strings.Index(url, "/")
	if idx < 0 {
		return "", ""
	}
	return url[:idx], url[idx+1:]
}

func gitClone(url, dest string) error {
	cmd := exec.Command("git", "clone", url, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

func gitFetch(dir string) error {
	cmd := exec.Command("git", "fetch", "origin", "--tags", "--prune")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

func gitResetHard(dir, ref string) error {
	cmd := exec.Command("git", "reset", "--hard", ref)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

func gitCheckout(dir, ref string) error {
	cmd := exec.Command("git", "checkout", ref)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(out))
	}
	return nil
}

// gitDefaultBranch detects the default branch name (main/master).
func gitDefaultBranch(dir string) string {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		return filepath.Base(ref)
	}
	// Fallback: check if origin/main exists
	cmd = exec.Command("git", "rev-parse", "--verify", "origin/main")
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		return "main"
	}
	return "master"
}
