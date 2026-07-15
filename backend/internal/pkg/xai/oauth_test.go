//go:build unit

package xai

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/stretchr/testify/require"
)

func TestBuildGrokMediaURLs(t *testing.T) {
	imagesURL, err := BuildImagesGenerationsURL(DefaultBaseURL + "/")
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL+"/images/generations", imagesURL)

	editsURL, err := BuildImagesEditsURL(DefaultBaseURL)
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL+"/images/edits", editsURL)

	videosURL, err := BuildVideosGenerationsURL(DefaultBaseURL)
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL+"/videos/generations", videosURL)

	videoEditsURL, err := BuildVideosEditsURL(DefaultBaseURL)
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL+"/videos/edits", videoEditsURL)

	videoExtensionsURL, err := BuildVideosExtensionsURL(DefaultBaseURL)
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL+"/videos/extensions", videoExtensionsURL)

	videoURL, err := BuildVideoURL(DefaultBaseURL, "req 123")
	require.NoError(t, err)
	require.Equal(t, DefaultBaseURL+"/videos/req%20123", videoURL)

	_, err = BuildVideoURL(DefaultBaseURL, " ")
	require.Error(t, err)
}

func TestValidateBaseURLPathPrefixPolicy(t *testing.T) {
	// 非官方主机保留管理员配置的任意 path 前缀。
	prefixed, err := ValidateBaseURL("https://relay.example.test/xai/v1/")
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.test/xai/v1", prefixed)

	deepPrefixed, err := ValidateBaseURL("https://relay.example.test/tenant-a/proxy")
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.test/tenant-a/proxy", deepPrefixed)

	// 空 path 仍按惯例补 /v1，保持既有配置兼容。
	rootOnly, err := ValidateBaseURL("https://relay.example.test")
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.test/v1", rootOnly)

	// 官方主机固定 /v1 前缀。
	_, err = ValidateBaseURL("https://api.x.ai/xai/v1")
	require.Error(t, err)
	_, err = ValidateBaseURL("https://cli-chat-proxy.grok.com/other")
	require.Error(t, err)
}

func TestValidateBaseURLsRejectEmptyQueryDelimiter(t *testing.T) {
	_, err := ValidateBaseURL("https://grok.example.test/v1?")
	require.Error(t, err)
}

func TestBuildResponsesURLWithValidatorUsesCallerPolicy(t *testing.T) {
	validator := func(raw string) (string, error) {
		return urlvalidator.ValidateURLFormat(raw, true)
	}

	target, err := BuildResponsesURLWithValidator("http://grok.example.test/v1/", validator)
	require.NoError(t, err)
	require.Equal(t, "http://grok.example.test/v1/responses", target)
}

func TestBuildResponsesURLPreservesUnsafeOverrideCustomPath(t *testing.T) {
	t.Setenv(EnvAllowUnsafeURLOverrides, "true")

	target, err := BuildResponsesURL("http://localhost:8080/custom")
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8080/custom/responses", target)
}

func TestBuildResponsesURLWithValidatorRejectsBaseURLComponents(t *testing.T) {
	permissive := func(raw string) (string, error) { return raw, nil }
	tests := []struct {
		name string
		raw  string
	}{
		{name: "userinfo", raw: "https://user:secret@grok.example.test/v1"},
		{name: "query", raw: "https://grok.example.test/v1?token=secret"},
		{name: "empty query delimiter", raw: "https://grok.example.test/v1?"},
		{name: "fragment", raw: "https://grok.example.test/v1#secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildResponsesURLWithValidator(tt.raw, permissive)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestDefaultModelMappingIncludesGrokAliases(t *testing.T) {
	t.Parallel()

	mapping := DefaultModelMapping()
	require.Equal(t, "grok-4.5", mapping["grok"])
	require.Equal(t, "grok-4.5", mapping["grok-latest"])
	require.Equal(t, "grok-4.5", mapping["grok-4.5"])
	require.Equal(t, "grok-4.5", mapping["grok-4.5-latest"])
	require.Equal(t, "grok-build-0.1", mapping["grok-build"])
	require.Equal(t, "grok-4.5", mapping["grok-build-latest"])
	require.Equal(t, "grok-composer-2.5-fast", mapping["grok-composer"])
	require.Equal(t, "grok-composer-2.5-fast", mapping["composer-2.5"])
	require.Equal(t, "grok-4.20-0309-reasoning", mapping["grok-4.20-reasoning"])
	require.Equal(t, "grok-4.20-0309-non-reasoning", mapping["grok-4.20-non-reasoning"])
	require.Equal(t, "grok-4.20-multi-agent-0309", mapping["grok-4.20-multi-agent-0309"])
	require.Equal(t, "grok-imagine", mapping["grok-imagine"])
	require.Equal(t, "grok-imagine-image", mapping["grok-imagine-image"])
	require.Equal(t, "grok-imagine-image-quality", mapping["grok-imagine-image-quality"])
	require.Equal(t, "grok-imagine-edit", mapping["grok-imagine-edit"])
	require.Equal(t, "grok-imagine-video", mapping["grok-imagine-video"])
	require.Equal(t, "grok-imagine-video-1.5", mapping["grok-imagine-video-1.5"])
}
