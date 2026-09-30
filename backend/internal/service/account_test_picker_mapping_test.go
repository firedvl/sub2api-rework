package service

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountTestPickerMappingUsesRawCache(t *testing.T) {
	var calls atomic.Int32
	gateway := newCodexModelsAPIKeyTestService(&codexModelsHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		calls.Add(1)
		return ordinaryModelsUpstreamResponse(`{"data":[{"id":"target","owned_by":"provider","created":123},{"id":"gpt-a"},{"id":"gpt-special"},{"id":"other","owned_by":"other-provider","created":456}]}`), nil
	}})
	svc := &AccountTestService{openaiGatewayService: gateway}
	account := newCodexModelsAPIKeyTestAccount("https://models.example/v1")
	ctx := context.Background()
	before, err := gateway.FetchOpenAIModelsList(ctx, account)
	require.NoError(t, err)
	account.Credentials["model_mapping"] = map[string]any{"gpt-*": "target", "gpt-special": "other"}
	models, err := svc.FetchOpenAIAccountModels(ctx, account)
	require.NoError(t, err)
	require.Len(t, models, 2)
	for _, model := range models {
		switch model.ID {
		case "gpt-a":
			require.Equal(t, "provider", model.OwnedBy)
			require.EqualValues(t, 123, model.Created)
		case "gpt-special":
			require.Equal(t, "other-provider", model.OwnedBy)
			require.EqualValues(t, 456, model.Created)
		default:
			t.Fatalf("unexpected unconfigured picker model %q", model.ID)
		}
	}
	account.Credentials["model_mapping"] = map[string]any{"second": "other"}
	models, err = svc.FetchOpenAIAccountModels(ctx, account)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "second", models[0].ID)
	account.Extra = map[string]any{"openai_passthrough": true}
	models, err = svc.FetchOpenAIAccountModels(ctx, account)
	require.NoError(t, err)
	require.Len(t, models, 4)
	for _, model := range models {
		require.NotEqual(t, "second", model.ID)
	}
	after, err := gateway.FetchOpenAIModelsList(ctx, account)
	require.NoError(t, err)
	require.Equal(t, before.Body, after.Body)
	require.EqualValues(t, 1, calls.Load())
}

func TestAccountTestPickerOAuthOnlyAddsConfiguredImageChoices(t *testing.T) {
	newCodexModelsOAuthCacheServer(t, `{"models":[{"slug":"discovered-text"}]}`)
	svc := &AccountTestService{openaiGatewayService: &OpenAIGatewayService{}}
	for _, scenario := range []struct {
		name        string
		mapping     map[string]any
		passthrough bool
		want        []string
	}{
		{"empty", nil, false, []string{"discovered-text"}},
		{"image_alias", map[string]any{"paint": "gpt-image-2.5-flare"}, false, []string{"paint"}},
		{"native_image", map[string]any{"gpt-image-2.5-flare": "gpt-image-2.5-flare"}, false, []string{"gpt-image-2.5-flare"}},
		{"lookalike", map[string]any{"gpt-image-lookalike": "undiscovered-text"}, false, nil},
		{"wildcard", map[string]any{"paint-*": "gpt-image-2.5-flare"}, false, nil},
		{"passthrough_alias", map[string]any{"paint": "gpt-image-2.5-flare"}, true, []string{"discovered-text"}},
		{"passthrough_native", map[string]any{"gpt-image-2.5-flare": "discovered-text"}, true, []string{"discovered-text", "gpt-image-2.5-flare"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			account := newCodexModelsTestAccount()
			account.Credentials["model_mapping"] = scenario.mapping
			account.Extra = map[string]any{"openai_passthrough": scenario.passthrough}
			before, err := svc.openaiGatewayService.FetchOpenAIModelsList(context.Background(), account)
			require.NoError(t, err)
			models, err := svc.FetchOpenAIAccountModels(context.Background(), account)
			require.NoError(t, err)
			ids := make([]string, 0, len(models))
			for _, model := range models {
				ids = append(ids, model.ID)
			}
			require.ElementsMatch(t, scenario.want, ids)
			after, err := svc.openaiGatewayService.FetchOpenAIModelsList(context.Background(), account)
			require.NoError(t, err)
			require.Equal(t, before.Body, after.Body)
		})
	}
}

func TestAccountTestPickerPassthroughChoiceUsesNativeModel(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"draw"}`)
	ctx, _ := newOpenAIImagesTestContext(t, body)
	upstream := &httpUpstreamRecorder{resp: openAIImagesJSONResponse()}
	svc := &AccountTestService{httpUpstream: upstream, cfg: &config.Config{}}
	account := newOpenAIImagesAPIKeyAccount()
	account.Extra = map[string]any{"openai_passthrough": true}
	account.Credentials["model_mapping"] = map[string]any{"gpt-image-2": "stale-text-target"}
	require.NoError(t, svc.testOpenAIAccountConnection(ctx, account, "gpt-image-2", "draw", ""))
	require.Equal(t, "/v1/images/generations", upstream.lastReq.URL.Path)
	require.Equal(t, "gpt-image-2", gjson.GetBytes(upstream.lastBody, "model").String())
}
