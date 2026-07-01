//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateServiceCacheStub struct {
	data string
}

func (s *updateServiceCacheStub) GetUpdateInfo(context.Context) (string, error) {
	if s.data == "" {
		return "", errors.New("cache miss")
	}
	return s.data, nil
}

func (s *updateServiceCacheStub) SetUpdateInfo(_ context.Context, data string, _ time.Duration) error {
	s.data = data
	return nil
}

type updateServiceGitHubClientStub struct {
	release *GitHubRelease
	repos   []string
}

func (s *updateServiceGitHubClientStub) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	s.repos = append(s.repos, repo)
	return s.release, nil
}

func (s *updateServiceGitHubClientStub) DownloadFile(context.Context, string, string, int64) error {
	panic("DownloadFile should not be called when no update is available")
}

func (s *updateServiceGitHubClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("FetchChecksumFile should not be called when no update is available")
}

func TestUpdateServicePerformUpdateNoUpdateReturnsSentinel(t *testing.T) {
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		&updateServiceGitHubClientStub{
			release: &GitHubRelease{
				TagName: "v0.1.132",
				Name:    "v0.1.132",
			},
		},
		"0.1.132",
		"release",
	)

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoUpdateAvailable))
	require.ErrorIs(t, err, ErrNoUpdateAvailable)
}

func TestUpdateServiceCheckUpdateUsesSecondaryDevelopmentRepository(t *testing.T) {
	client := &updateServiceGitHubClientStub{
		release: &GitHubRelease{
			TagName: "v1.4.1",
			Name:    "v1.4.1",
			HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/v1.4.1",
		},
	}
	svc := NewUpdateService(
		&updateServiceCacheStub{},
		client,
		"1.4.0",
		"release",
	)

	info, err := svc.CheckUpdate(context.Background(), true)

	require.NoError(t, err)
	require.Equal(t, []string{"thornboo/sub2api"}, client.repos)
	require.True(t, info.HasUpdate)
	require.Equal(t, "1.4.1", info.LatestVersion)
	require.NotNil(t, info.ReleaseInfo)
	require.Equal(t, "https://github.com/thornboo/sub2api/releases/tag/v1.4.1", info.ReleaseInfo.HTMLURL)
}

func TestUpdateServiceCheckUpdateIgnoresCachedUpstreamRepository(t *testing.T) {
	cached, err := json.Marshal(struct {
		Latest      string       `json:"latest"`
		Repo        string       `json:"repo"`
		ReleaseInfo *ReleaseInfo `json:"release_info"`
		Timestamp   int64        `json:"timestamp"`
	}{
		Latest: "9.9.9",
		Repo:   "Wei-Shaw/sub2api",
		ReleaseInfo: &ReleaseInfo{
			HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/v9.9.9",
		},
		Timestamp: time.Now().Unix(),
	})
	require.NoError(t, err)

	client := &updateServiceGitHubClientStub{
		release: &GitHubRelease{
			TagName: "v1.4.1",
			Name:    "v1.4.1",
			HTMLURL: "https://github.com/thornboo/sub2api/releases/tag/v1.4.1",
		},
	}
	svc := NewUpdateService(
		&updateServiceCacheStub{data: string(cached)},
		client,
		"1.4.1",
		"release",
	)

	info, err := svc.CheckUpdate(context.Background(), false)

	require.NoError(t, err)
	require.Equal(t, []string{"thornboo/sub2api"}, client.repos)
	require.Equal(t, "1.4.1", info.LatestVersion)
	require.NotNil(t, info.ReleaseInfo)
	require.Equal(t, "https://github.com/thornboo/sub2api/releases/tag/v1.4.1", info.ReleaseInfo.HTMLURL)
}
