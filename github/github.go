package github

import (
	"context"
	"fmt"
	"time"

	"github.com/google/go-github/github"
)

// LatestRelease fetches the latest release from a GitHub repository.
func LatestRelease(org, repo string) (string, error) {
	client := github.NewClient(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	release, _, err := client.Repositories.GetLatestRelease(ctx, org, repo)
	if err != nil {
		return "", err
	} else if release.Name == nil {
		return "", fmt.Errorf("no release found for %s/%s", org, repo)
	}
	return *release.Name, nil
}
