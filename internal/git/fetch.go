package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// FetchKeyContext returns an empty key when checkout-specific fetch behavior
// cannot safely be shared with other worktrees.
func FetchKeyContext(ctx context.Context, repoPath string) string {
	for _, name := range []string{"GIT_SSH", "GIT_SSH_COMMAND", "GIT_PROXY_COMMAND", "GIT_ASKPASS", "SSH_ASKPASS"} {
		if os.Getenv(name) != "" {
			return ""
		}
	}
	for _, name := range []string{"GIT_SSL_CAINFO", "GIT_SSL_CAPATH", "GIT_SSL_CERT", "GIT_SSL_KEY"} {
		if value := os.Getenv(name); value != "" && !filepath.IsAbs(value) {
			return ""
		}
	}
	common := RunContext(ctx, repoPath, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if !common.OK {
		return ""
	}
	commonPath, err := filepath.EvalSymlinks(strings.TrimSuffix(common.Stdout, "\n"))
	if err != nil {
		return ""
	}
	config := RunContext(ctx, repoPath, "config", "--null", "--list", "--includes")
	if !config.OK {
		return ""
	}
	for _, entry := range strings.Split(config.Stdout, "\x00") {
		key, value, _ := strings.Cut(entry, "\n")
		key = strings.ToLower(key)
		if checkoutFetchConfig(key, value) {
			return ""
		}
	}
	for _, path := range []string{
		filepath.Join(repoPath, ".gitmodules"),
		filepath.Join(commonPath, "modules"),
		filepath.Join(commonPath, "hooks", "reference-transaction"),
		filepath.Join(commonPath, "hooks", "pre-auto-gc"),
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			return ""
		}
	}
	modules := RunContext(ctx, repoPath, "ls-files", "-z", "--", ".gitmodules")
	if !modules.OK || modules.Stdout != "" {
		return ""
	}
	return fmt.Sprintf("%s\x00%x", commonPath, sha256.Sum256([]byte(config.Stdout)))
}

func checkoutFetchConfig(key, value string) bool {
	if key == "extensions.worktreeconfig" || strings.HasPrefix(key, "includeif.") || strings.HasPrefix(key, "submodule.") {
		return true
	}
	if key == "core.sshcommand" || key == "core.gitproxy" || key == "core.askpass" || key == "core.alternaterefscommand" || key == "core.hookspath" ||
		strings.HasPrefix(key, "credential.") && strings.HasSuffix(key, ".helper") ||
		strings.HasPrefix(key, "url.") && strings.HasSuffix(key, ".insteadof") {
		return true
	}
	if (key == "fetch.fsck.skiplist" || key == "fsck.skiplist") && value != "" && !filepath.IsAbs(value) {
		return true
	}
	if key == "fetch.bundleuri" && !checkoutIndependentURL(value) {
		return true
	}
	if strings.HasPrefix(key, "remote.") {
		if strings.HasSuffix(key, ".uploadpack") || strings.HasSuffix(key, ".vcs") {
			return true
		}
		if strings.HasSuffix(key, ".url") {
			return !checkoutIndependentURL(value)
		}
	}
	if strings.HasPrefix(key, "http.") && value != "" && !filepath.IsAbs(value) {
		for _, suffix := range []string{".sslcainfo", ".sslcapath", ".sslcert", ".sslkey", ".proxysslcainfo", ".proxysslcert", ".proxysslkey", ".cookiefile"} {
			if strings.HasSuffix(key, suffix) {
				return true
			}
		}
	}
	return false
}

func checkoutIndependentURL(value string) bool {
	if strings.Contains(value, "::") {
		return false
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return false
		}
		switch parsed.Scheme {
		case "http", "https", "ssh", "git":
			return true
		case "file":
			return filepath.IsAbs(parsed.Path)
		default:
			return false
		}
	}
	if filepath.IsAbs(value) {
		return true
	}
	colon, slash := strings.IndexByte(value, ':'), strings.IndexByte(value, '/')
	return colon > 0 && (slash < 0 || colon < slash)
}
