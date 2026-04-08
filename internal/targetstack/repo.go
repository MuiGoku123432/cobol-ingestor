package targetstack

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
)

// CloneOrPull clones the repository if it doesn't exist locally, or pulls the
// latest changes if it does. Returns a CloneResult with the HEAD SHA.
func CloneOrPull(cfg RepoConfig, cloneBaseDir string, shallow bool, logger *zap.Logger) (*CloneResult, error) {
	localPath := repoLocalPath(cfg.URL, cloneBaseDir)

	if err := os.MkdirAll(localPath, 0o755); err != nil {
		return nil, fmt.Errorf("creating clone dir %s: %w", localPath, err)
	}

	var auth *http.BasicAuth
	if cfg.Token != "" {
		auth = tokenAuth(cfg.URL, cfg.Token)
	}

	// Check if already cloned
	repo, openErr := git.PlainOpen(localPath)
	if openErr != nil {
		// Not cloned yet — clone
		logger.Info("cloning repository", zap.String("url", cfg.URL), zap.String("path", localPath))

		cloneOpts := &git.CloneOptions{
			URL:  cfg.URL,
			Auth: auth,
		}
		if cfg.Branch != "" {
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(cfg.Branch)
		}
		if shallow {
			cloneOpts.Depth = 1
		}

		var err error
		repo, err = git.PlainClone(localPath, false, cloneOpts)
		if err != nil {
			return nil, fmt.Errorf("cloning %s: %w", cfg.URL, err)
		}
	} else {
		// Already cloned — pull
		logger.Info("pulling repository", zap.String("url", cfg.URL))

		wt, err := repo.Worktree()
		if err != nil {
			return nil, fmt.Errorf("getting worktree: %w", err)
		}

		pullOpts := &git.PullOptions{
			Auth: auth,
		}
		if cfg.Branch != "" {
			pullOpts.ReferenceName = plumbing.NewBranchReferenceName(cfg.Branch)
		}
		if shallow {
			pullOpts.Depth = 1
		}

		err = wt.Pull(pullOpts)
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return nil, fmt.Errorf("pulling %s: %w", cfg.URL, err)
		}
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("getting HEAD: %w", err)
	}

	return &CloneResult{
		Config:    cfg,
		LocalPath: localPath,
		HeadSHA:   head.Hash().String(),
	}, nil
}

// repoLocalPath derives a stable local directory from a repo URL.
// e.g., https://github.com/org/repo → <base>/github.com/org/repo
func repoLocalPath(repoURL, baseDir string) string {
	u, err := url.Parse(repoURL)
	if err != nil || u.Host == "" {
		// fallback: sanitize the raw URL
		safe := strings.NewReplacer("://", "_", "/", "_", ":", "_").Replace(repoURL)
		return filepath.Join(baseDir, safe)
	}
	// Strip .git suffix from path
	p := strings.TrimSuffix(u.Path, ".git")
	p = strings.TrimPrefix(p, "/")
	return filepath.Join(baseDir, u.Host, p)
}

// tokenAuth builds the appropriate BasicAuth for the provider.
// GitHub uses token as the password with "x-access-token" as username.
// Azure DevOps uses token as the password with any username (e.g. "pat").
func tokenAuth(repoURL, token string) *http.BasicAuth {
	if strings.Contains(repoURL, "dev.azure.com") || strings.Contains(repoURL, "visualstudio.com") {
		return &http.BasicAuth{Username: "pat", Password: token}
	}
	return &http.BasicAuth{Username: "x-access-token", Password: token}
}

// DetectProvider infers the Git provider from a repo URL.
func DetectProvider(repoURL string) string {
	switch {
	case strings.Contains(repoURL, "github.com"):
		return "github"
	case strings.Contains(repoURL, "dev.azure.com"), strings.Contains(repoURL, "visualstudio.com"):
		return "azure_devops"
	default:
		return "generic"
	}
}

// RepoName extracts a human-readable name from a repo URL.
func RepoName(repoURL string) string {
	u, err := url.Parse(repoURL)
	if err != nil {
		return repoURL
	}
	parts := strings.Split(strings.TrimSuffix(u.Path, ".git"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return repoURL
}
