package plugins

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var expectedKeys = []string{"alibaba", "doubao", "google", "hailuo", "jimeng", "kling", "sora", "sunoapi", "tencent-vega", "vertex-ai", "vidu", "xai"}

func TestBuiltInVendorPluginsDeclareNativeRoutesAndLegacyChannelTypes(t *testing.T) {
	generation := jsplugin.DefaultRegistry.Generation()
	require.NotNil(t, generation)

	routes := []struct {
		method    string
		path      string
		key       string
		routeType jsplugin.RouteType
		action    string
		renderer  string
	}{
		{"POST", "/kling/v1/videos/text2video", "kling", jsplugin.RouteTypeSubmit, "text_to_video", "taskCreated"},
		{"POST", "/kling/v1/videos/image2video", "kling", jsplugin.RouteTypeSubmit, "image_to_video", "taskCreated"},
		{"GET", "/kling/v1/videos/text2video/:task_id", "kling", jsplugin.RouteTypeQuery, "", "taskStatus"},
		{"GET", "/kling/v1/videos/image2video/:task_id", "kling", jsplugin.RouteTypeQuery, "", "taskStatus"},
		{"POST", "/jimeng/", "jimeng", jsplugin.RouteTypeDynamic, "", "renderTask"},
		{"POST", "/wand-vega/aigc-video-task", "tencent-vega", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"POST", "/wand-vega/aigc-material", "tencent-vega", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"GET", "/wand-vega/task/:task_id", "tencent-vega", jsplugin.RouteTypeQuery, "", "taskStatus"},
		{"POST", "/suno/submit/:action", "sunoapi", jsplugin.RouteTypeSubmit, "", "renderSubmit"},
		{"POST", "/suno/fetch", "sunoapi", jsplugin.RouteTypeDynamic, "", "renderTasks"},
		{"GET", "/suno/fetch/:task_id", "sunoapi", jsplugin.RouteTypeQuery, "", "renderTask"},
		{"POST", "/doubao/api/v3/contents/generations/tasks", "doubao", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"GET", "/doubao/api/v3/contents/generations/tasks/:task_id", "doubao", jsplugin.RouteTypeQuery, "", "taskStatus"},
		{"POST", "/doubao/api/v3/images/generations", "doubao", jsplugin.RouteTypeSubmit, "", "imageCreated"},
		{"POST", "/xai/v1/videos/generations", "xai", jsplugin.RouteTypeSubmit, "", "videoCreated"},
		{"POST", "/xai/v1/videos/edits", "xai", jsplugin.RouteTypeSubmit, "", "videoCreated"},
		{"POST", "/xai/v1/videos/extensions", "xai", jsplugin.RouteTypeSubmit, "", "videoCreated"},
		{"GET", "/xai/v1/videos/:request_id", "xai", jsplugin.RouteTypeQuery, "", "videoStatus"},
		{"POST", "/hailuo/v1/video_generation", "hailuo", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"GET", "/hailuo/v1/query/video_generation/:task_id", "hailuo", jsplugin.RouteTypeQuery, "", "taskStatus"},
		{"POST", "/hailuo/v2/video_generation", "hailuo", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"GET", "/hailuo/v2/query/video_generation/:task_id", "hailuo", jsplugin.RouteTypeQuery, "", "h3TaskStatus"},
		{"POST", "/vidu/ent/v2/text2video", "vidu", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"POST", "/vidu/ent/v2/img2video", "vidu", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"POST", "/vidu/ent/v2/start-end2video", "vidu", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"POST", "/vidu/ent/v2/reference2video", "vidu", jsplugin.RouteTypeSubmit, "", "taskCreated"},
		{"GET", "/vidu/ent/v2/tasks/:task_id/creations", "vidu", jsplugin.RouteTypeQuery, "", "taskStatus"},
		{"POST", "/google/v1beta/models/:model", "google", jsplugin.RouteTypeSubmit, "", "operation"},
		{"GET", "/google/v1beta/models/:model/operations/:task_id", "google", jsplugin.RouteTypeQuery, "", "operation"},
		{"POST", "/vertex-ai/v1/projects/:project/locations/:location/publishers/google/models/:model", "vertex-ai", jsplugin.RouteTypeDynamic, "", "operation"},
	}
	for _, expected := range routes {
		t.Run(expected.method+" "+expected.path, func(t *testing.T) {
			binding, found := generation.LookupDeclaredRoute(expected.method, expected.path)
			require.True(t, found)
			require.Equal(t, expected.key, binding.Plugin.Meta.Key)
			require.Equal(t, expected.routeType, binding.Route.Type)
			require.Equal(t, expected.action, binding.Route.Action)
			require.Equal(t, expected.renderer, binding.Route.Render)
		})
	}

	// The legacy Wan edit route serves both the wan2.5 image-list body and the
	// wanx2.1-imageedit function API.
	imageEdit, found := generation.LookupDeclaredRoute("POST", "/ali/api/v1/services/aigc/image2image/image-synthesis")
	require.True(t, found)
	assert.Equal(t, "alibaba", imageEdit.Plugin.Meta.Key)
	assert.ElementsMatch(t, []string{"wan2.5-i2i-preview", "wanx2.1-imageedit"}, imageEdit.Route.Models)

	channelTypes := []struct {
		value int
		key   string
	}{
		{1, "sora"},
		{36, "sunoapi"},
		{45, "doubao"},
		{50, "kling"},
		{51, "jimeng"},
		{54, "doubao"},
		{55, "sora"},
		{48, "xai"},
	}
	for _, channelType := range channelTypes {
		plugin, found := generation.GetByChannelType(channelType.value)
		require.True(t, found)
		require.Equal(t, channelType.key, plugin.Meta.Key)
	}
}

func TestBuiltInTaskPluginResponsesAndUsageContracts(t *testing.T) {
	generation := jsplugin.DefaultRegistry.Generation()
	require.NotNil(t, generation)

	entries, err := fs.ReadDir(taskPlugins, "tasks")
	require.NoError(t, err)
	actualKeys := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			actualKeys = append(actualKeys, entry.Name())
		}
	}
	assert.Equal(t, expectedKeys, actualKeys)

	for _, key := range expectedKeys {
		t.Run(key, func(t *testing.T) {
			_, found := generation.Get(key)
			require.True(t, found, "factory plugin was excluded from the active generation")

			source, sourceErr := Source(key)
			require.NoError(t, sourceErr)
			registry := jsplugin.NewRegistry()
			plugin, registerErr := registry.RegisterFactory(source, jsplugin.Options{Key: key})
			require.NoError(t, registerErr)

			var responsesClaim jsplugin.ProtocolClaim
			foundResponses := false
			for _, claim := range plugin.Meta.Protocols {
				if claim.Name == "openai_responses" {
					responsesClaim = claim
					foundResponses = true
					break
				}
			}
			require.True(t, foundResponses, "openai_responses claim must be present")
			assert.Equal(t, []string{"stream", "sync", "background"}, responsesClaim.Supports)
			for _, model := range plugin.Meta.Models {
				binding, claimed := registry.Generation().LookupEndpoint("POST", "/v1/responses", model)
				require.True(t, claimed, model)
				assert.Same(t, plugin, binding.Plugin)
			}
			for _, hook := range []string{"decodeRequest", "renderEvents", "renderFinal"} {
				callable, callableErr := plugin.Engine.HasCallablePath(t.Context(), "protocols", "openai_responses", hook)
				require.NoError(t, callableErr)
				assert.True(t, callable, hook)
			}
			for _, hook := range []string{"extractUsage", "extractUsageOnComplete"} {
				assert.True(t, plugin.Engine.HasExport(hook), hook)
			}
			require.NotEmpty(t, plugin.Meta.UsageSchema)
			for usageKey, schema := range plugin.Meta.UsageSchema {
				assert.NotEmpty(t, schema.Description, usageKey)
			}
		})
	}
}

func TestBuiltInResponsesDecodersEchoChannelMappedAlias(t *testing.T) {
	bodyOverrides := map[string]map[string]any{}
	for _, key := range expectedKeys {
		t.Run(key, func(t *testing.T) {
			source, sourceErr := Source(key)
			require.NoError(t, sourceErr)
			registry := jsplugin.NewRegistry()
			plugin, registerErr := registry.RegisterFactory(source, jsplugin.Options{Key: key})
			require.NoError(t, registerErr)
			require.NotEmpty(t, plugin.Meta.Models)

			alias := "alias-under-test"
			upstreamModel := plugin.Meta.Models[0]
			body := map[string]any{"model": alias, "input": "a cat walking on the beach"}
			if override, ok := bodyOverrides[key]; ok {
				body = override
			}
			value, callErr := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_responses", "decodeRequest"}, map[string]any{
				"model": alias, "upstreamModel": upstreamModel, "stream": false,
				"body": map[string]any{"kind": "json", "value": body},
			})
			require.NoError(t, callErr)
			encoded, marshalErr := common.Marshal(value)
			require.NoError(t, marshalErr)
			var decoded map[string]any
			require.NoError(t, common.Unmarshal(encoded, &decoded))
			assert.Equal(t, "submit", decoded["kind"])
			assert.Equal(t, alias, decoded["model"])
		})
	}
}

func TestBuiltInPluginsAddressNewAPIUpstreamOnNativeRoutes(t *testing.T) {
	generation := jsplugin.DefaultRegistry.Generation()
	require.NotNil(t, generation)

	const base = "https://upstream.example"
	cases := []struct {
		key                string
		driver             map[string]any
		vendorKey          string
		vendorAuthPrefix   string
		vendorSubmit       string
		gatewaySubmit      string
		vendorQuery        string
		gatewayQuery       string
		content            string
		legacyKeyHeuristic bool
	}{
		{
			key:              "doubao",
			driver:           map[string]any{"action": "text_to_video", "model": "doubao-seedance-1-0-pro-250528", "upstreamModel": "doubao-seedance-1-0-pro-250528", "requestBody": map[string]any{"model": "doubao-seedance-1-0-pro-250528", "prompt": "a cat"}},
			vendorKey:        "vendor-key",
			vendorAuthPrefix: "Bearer vendor-key",
			vendorSubmit:     "/api/v3/contents/generations/tasks",
			gatewaySubmit:    "/doubao/api/v3/contents/generations/tasks",
			vendorQuery:      "/api/v3/contents/generations/tasks/tid",
			gatewayQuery:     "/doubao/api/v3/contents/generations/tasks/tid",
		},
		{
			key:              "alibaba",
			driver:           map[string]any{"action": "text_to_video", "model": "wan2.2-t2v-plus", "upstreamModel": "wan2.2-t2v-plus", "requestBody": map[string]any{"model": "wan2.2-t2v-plus", "prompt": "a cat"}},
			vendorKey:        "vendor-key",
			vendorAuthPrefix: "Bearer vendor-key",
			vendorSubmit:     "/api/v1/services/aigc/video-generation/video-synthesis",
			gatewaySubmit:    "/ali/api/v1/services/aigc/video-generation/video-synthesis",
			vendorQuery:      "/api/v1/tasks/tid",
			gatewayQuery:     "/ali/api/v1/tasks/tid",
		},
		{
			key:                "kling",
			driver:             map[string]any{"action": "text_to_video", "model": "kling-v1", "upstreamModel": "kling-v1", "requestBody": map[string]any{"prompt": "a cat"}},
			vendorKey:          "ak|sk",
			vendorAuthPrefix:   "Bearer ey",
			vendorSubmit:       "/v1/videos/text2video",
			gatewaySubmit:      "/kling/v1/videos/text2video",
			vendorQuery:        "/v1/videos/text2video/tid",
			gatewayQuery:       "/kling/v1/videos/text2video/tid",
			legacyKeyHeuristic: true,
		},
		{
			key:                "jimeng",
			driver:             map[string]any{"action": "text_to_video", "model": "jimeng_vgfm_t2v_l20", "upstreamModel": "jimeng_vgfm_t2v_l20", "requestBody": map[string]any{"prompt": "a cat"}},
			vendorKey:          "ak|sk",
			vendorAuthPrefix:   "HMAC-SHA256",
			vendorSubmit:       "/?Action=CVSync2AsyncSubmitTask&Version=2022-08-31",
			gatewaySubmit:      "/jimeng/?Action=CVSync2AsyncSubmitTask&Version=2022-08-31",
			vendorQuery:        "/?Action=CVSync2AsyncGetResult&Version=2022-08-31",
			gatewayQuery:       "/jimeng/?Action=CVSync2AsyncGetResult&Version=2022-08-31",
			legacyKeyHeuristic: true,
		},
		{
			key:              "sunoapi",
			driver:           map[string]any{"action": "MUSIC", "model": "suno_music", "upstreamModel": "suno_music", "requestBody": map[string]any{"prompt": "a song"}},
			vendorKey:        "vendor-key",
			vendorAuthPrefix: "Bearer vendor-key",
			vendorSubmit:     "/suno/submit/MUSIC",
			gatewaySubmit:    "/suno/submit/MUSIC",
			vendorQuery:      "/suno/fetch",
			gatewayQuery:     "/suno/fetch",
		},
		{
			key:              "sora",
			driver:           map[string]any{"action": "text_to_video", "model": "sora-2", "upstreamModel": "sora-2", "requestBody": map[string]any{"prompt": "a cat"}},
			vendorKey:        "vendor-key",
			vendorAuthPrefix: "Bearer vendor-key",
			vendorSubmit:     "/v1/videos",
			gatewaySubmit:    "/v1/videos",
			vendorQuery:      "/v1/videos/tid",
			gatewayQuery:     "/v1/videos/tid",
			content:          "/v1/videos/tid/content",
		},
		{
			key:              "xai",
			driver:           map[string]any{"action": "text_to_video", "model": "grok-imagine-video", "upstreamModel": "grok-imagine-video", "requestBody": map[string]any{"model": "grok-imagine-video", "prompt": "a cat"}},
			vendorKey:        "vendor-key",
			vendorAuthPrefix: "Bearer vendor-key",
			vendorSubmit:     "/v1/videos/generations",
			gatewaySubmit:    "/xai/v1/videos/generations",
			vendorQuery:      "/v1/videos/tid",
			gatewayQuery:     "/xai/v1/videos/tid",
		},
		{
			key:              "hailuo",
			driver:           map[string]any{"action": "text_to_video", "model": "MiniMax-Hailuo-2.3", "upstreamModel": "MiniMax-Hailuo-2.3", "requestBody": map[string]any{"model": "MiniMax-Hailuo-2.3", "prompt": "a cat"}},
			vendorKey:        "vendor-key",
			vendorAuthPrefix: "Bearer vendor-key",
			vendorSubmit:     "/v1/video_generation",
			gatewaySubmit:    "/hailuo/v1/video_generation",
			vendorQuery:      "/v1/query/video_generation?task_id=tid",
			gatewayQuery:     "/hailuo/v1/query/video_generation/tid",
		},
		{
			key:              "vidu",
			driver:           map[string]any{"action": "text_to_video", "model": "viduq2", "upstreamModel": "viduq2", "requestBody": map[string]any{"model": "viduq2", "prompt": "a cat"}},
			vendorKey:        "vendor-key",
			vendorAuthPrefix: "Token vendor-key",
			vendorSubmit:     "/ent/v2/text2video",
			gatewaySubmit:    "/vidu/ent/v2/text2video",
			vendorQuery:      "/ent/v2/tasks/tid/creations",
			gatewayQuery:     "/vidu/ent/v2/tasks/tid/creations",
		},
		{
			key:              "tencent-vega",
			driver:           map[string]any{"action": "text_to_video", "model": "wand-vega-video-1.0-pro", "upstreamModel": "wand-vega-video-1.0-pro", "requestBody": map[string]any{"model": "wand-vega-video-1.0-pro", "prompt": "a cat"}},
			vendorKey:        "sid|sk|150001",
			vendorAuthPrefix: "TC3-HMAC-SHA256",
			vendorSubmit:     "/",
			gatewaySubmit:    "/wand-vega/aigc-video-task",
			vendorQuery:      "/",
			gatewayQuery:     "/wand-vega/task/tid",
		},
	}
	variants := []struct {
		name      string
		upstream  map[string]any
		gateway   bool
		legacyKey bool
	}{
		{"vendor signal", map[string]any{"kind": "vendor"}, false, false},
		{"no signal from an older host", nil, false, false},
		{"new_api signal", map[string]any{"kind": "new_api"}, true, false},
		{"legacy sk- key without a signal", nil, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			plugin, found := generation.Get(tc.key)
			require.True(t, found)
			require.True(t, plugin.Meta.SupportsUpstream(jsplugin.UpstreamKindNewAPI), "%s must declare new_api to be bindable to a New API channel", tc.key)
			call := func(hook string, args ...any) map[string]any {
				value, err := plugin.Engine.Call(t.Context(), hook, args...)
				require.NoError(t, err, hook)
				encoded, err := common.Marshal(value)
				require.NoError(t, err)
				var descriptor map[string]any
				require.NoError(t, common.Unmarshal(encoded, &descriptor))
				return descriptor
			}
			for _, variant := range variants {
				if variant.legacyKey && !tc.legacyKeyHeuristic {
					continue
				}
				t.Run(variant.name, func(t *testing.T) {
					apiKey := tc.vendorKey
					if variant.legacyKey {
						apiKey = "sk-legacy"
					} else if variant.gateway {
						apiKey = "sk-gateway"
					}
					authHeader := apiKey
					if variant.gateway && !variant.legacyKey {
						authHeader = "Bearer " + apiKey
					}
					ctx := map[string]any{"baseUrl": base, "apiKey": apiKey, "authHeader": authHeader, "publicTaskId": "task_pub", "requestHeaders": map[string]any{}, "files": []any{}}
					for key, value := range tc.driver {
						ctx[key] = value
					}
					if variant.upstream != nil {
						ctx["upstream"] = variant.upstream
					}
					wantSubmit, wantQuery := tc.vendorSubmit, tc.vendorQuery
					if variant.gateway {
						wantSubmit, wantQuery = tc.gatewaySubmit, tc.gatewayQuery
					}

					submit := call("buildSubmitRequest", ctx)
					assert.Equal(t, base+wantSubmit, submit["url"])
					headers, _ := submit["headers"].(map[string]any)
					authorization, _ := headers["Authorization"].(string)
					if variant.gateway {
						assert.Equal(t, "Bearer "+apiKey, authorization, "a gateway receives its own token as a Bearer header")
					} else {
						assert.True(t, strings.HasPrefix(authorization, tc.vendorAuthPrefix), "vendor Authorization %q", authorization)
					}

					queryCtx := map[string]any{"taskId": "tid", "publicTaskId": "task_pub", "action": tc.driver["action"], "model": tc.driver["model"], "upstreamModel": tc.driver["upstreamModel"], "baseUrl": base, "apiKey": apiKey, "authHeader": authHeader, "data": nil, "state": nil}
					if variant.upstream != nil {
						queryCtx["upstream"] = variant.upstream
					}
					var query map[string]any
					if plugin.Meta.FetchMode == "batch" {
						query = call("buildBatchQueryRequest", queryCtx, []any{queryCtx})
					} else {
						query = call("buildQueryRequest", queryCtx)
					}
					assert.Equal(t, base+wantQuery, query["url"])

					if tc.content == "" {
						return
					}
					contentCtx := map[string]any{"baseUrl": base, "apiKey": apiKey, "authHeader": authHeader, "artifactKey": "video", "upstreamTaskId": "tid", "data": map[string]any{}, "clientRequest": map[string]any{"method": "GET", "headers": map[string]any{}}}
					if variant.upstream != nil {
						contentCtx["upstream"] = variant.upstream
					}
					assert.Equal(t, base+tc.content, call("buildContentRequest", contentCtx)["url"])
				})
			}
		})
	}

	t.Run("alibaba asks only DashScope for an interleaved image stream", func(t *testing.T) {
		plugin, found := generation.Get("alibaba")
		require.True(t, found)
		for _, variant := range []struct {
			name             string
			upstream         map[string]any
			wantResponseType string
			wantSSEHeader    bool
		}{
			{"vendor", map[string]any{"kind": "vendor"}, "sse", true},
			{"new_api", map[string]any{"kind": "new_api"}, "json", false},
		} {
			t.Run(variant.name, func(t *testing.T) {
				value, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
					"baseUrl": base, "apiKey": "key", "authHeader": "key", "publicTaskId": "task_pub", "upstream": variant.upstream,
					"model": "wan2.6-image", "upstreamModel": "wan2.6-image",
					"requestBody": map[string]any{"model": "wan2.6-image", "prompt": "a cat", "metadata": map[string]any{"upstream_mode": "sync", "parameters": map[string]any{"enable_interleave": true}}},
				})
				require.NoError(t, err)
				encoded, err := common.Marshal(value)
				require.NoError(t, err)
				var descriptor struct {
					URL          string            `json:"url"`
					ResponseType string            `json:"responseType"`
					Headers      map[string]string `json:"headers"`
				}
				require.NoError(t, common.Unmarshal(encoded, &descriptor))
				assert.Equal(t, variant.wantResponseType, descriptor.ResponseType, "a gateway native route aggregates the vendor stream and answers with JSON")
				_, hasSSEHeader := descriptor.Headers["X-DashScope-Sse"]
				assert.Equal(t, variant.wantSSEHeader, hasSSEHeader)
			})
		}
	})
}

func TestTencentVegaDecodesContentAndParsesVODStatus(t *testing.T) {
	plugin, found := jsplugin.DefaultRegistry.Generation().Get("tencent-vega")
	require.True(t, found)

	decoded, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
		"model": "wand-vega-video-1.0-pro", "upstreamModel": "wand-vega-video-1.0-pro",
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"model": "wand-vega-video-1.0-pro", "prompt": "他在散步", "seconds": 5,
			"metadata": map[string]any{
				"resolution": "720P",
				"content": []any{
					map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/a.png"}, "role": "first_frame"},
				},
			},
		}},
	})
	require.NoError(t, err)
	encoded, err := common.Marshal(decoded)
	require.NoError(t, err)
	var intent map[string]any
	require.NoError(t, common.Unmarshal(encoded, &intent))
	assert.Equal(t, "submit", intent["kind"])
	assert.Equal(t, "image_to_video", intent["action"])
	assert.NotContains(t, intent, "originTaskIds")

	submit, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
		"baseUrl": "https://vod.tencentcloudapi.com", "apiKey": "sid|sk|150001", "authHeader": "",
		"model": "wand-vega-video-1.0-pro", "upstreamModel": "wand-vega-video-1.0-pro",
		"requestBody": intent["requestBody"], "upstream": map[string]any{"kind": "vendor"},
		"publicTaskId": "task_pub", "requestHeaders": map[string]any{}, "files": []any{},
	})
	require.NoError(t, err)
	submitEncoded, err := common.Marshal(submit)
	require.NoError(t, err)
	var descriptor struct {
		URL     string         `json:"url"`
		Headers map[string]any `json:"headers"`
		Body    string         `json:"body"`
	}
	require.NoError(t, common.Unmarshal(submitEncoded, &descriptor))
	assert.Equal(t, "https://vod.tencentcloudapi.com/", descriptor.URL)
	assert.Equal(t, "CreateAigcVideoTask", descriptor.Headers["X-TC-Action"])
	assert.Contains(t, descriptor.Body, `"SubAppId":150001`)
	assert.Contains(t, descriptor.Body, `"Usage":"FirstFrame"`)
	assert.Contains(t, descriptor.Body, `"Duration":5`)

	reserved, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
		"baseUrl": "https://vod.tencentcloudapi.com", "apiKey": "sid|sk|150001", "authHeader": "",
		"model": "wand-vega-video-1.0-pro", "upstreamModel": "wand-vega-video-1.0-pro",
		"requestBody":  map[string]any{"model": "wand-vega-video-1.0-pro", "prompt": "他在散步", "seconds": -1},
		"upstream":     map[string]any{"kind": "vendor"},
		"publicTaskId": "task_pub", "requestHeaders": map[string]any{}, "files": []any{},
	})
	require.NoError(t, err)
	reservedEncoded, err := common.Marshal(reserved)
	require.NoError(t, err)
	var reservedDescriptor struct {
		Body string `json:"body"`
	}
	require.NoError(t, common.Unmarshal(reservedEncoded, &reservedDescriptor))
	assert.Contains(t, reservedDescriptor.Body, `"Duration":15`)

	parsed, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{"taskId": "tid"}, map[string]any{
		"Response": map[string]any{
			"Status": "FINISH",
			"AigcVideoTask": map[string]any{
				"Status": "FINISH",
				"Output": map[string]any{"FileInfos": []any{map[string]any{"FileUrl": "https://vod.example/a.mp4", "UsageType": ""}}},
			},
		},
	})
	require.NoError(t, err)
	parsedEncoded, err := common.Marshal(parsed)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, common.Unmarshal(parsedEncoded, &result))
	assert.Equal(t, "SUCCESS", result["status"])
	assert.Equal(t, "https://vod.example/a.mp4", result["url"])

	completed, err := plugin.Engine.Call(t.Context(), "extractUsageOnComplete", map[string]any{"taskId": "tid"}, map[string]any{}, map[string]any{
		"Response": map[string]any{
			"AigcVideoTask": map[string]any{
				"Status": "FINISH",
				"Output": map[string]any{
					"Usage": map[string]any{"InputSeconds": 4, "OutputSeconds": 4, "TotalSeconds": 8},
					"FileInfos": []any{map[string]any{
						"FileUrl": "https://vod.example/a.mp4", "UsageType": "",
						"MetaData": map[string]any{"Duration": 4.042, "Width": 720, "Height": 1280},
					}},
				},
			},
		},
	})
	require.NoError(t, err)
	completedEncoded, err := common.Marshal(completed)
	require.NoError(t, err)
	var completedFacts map[string]any
	require.NoError(t, common.Unmarshal(completedEncoded, &completedFacts))
	assert.Equal(t, map[string]any{"seconds": float64(8), "resolution": "720P"}, completedFacts)

	liteImages := make([]any, 0, 6)
	for i := range 6 {
		liteImages = append(liteImages, map[string]any{"type": "image_url", "image_url": map[string]any{"url": fmt.Sprintf("https://example.com/%d.png", i)}})
	}
	liteDecoded, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
		"model": "wand-vega-video-1.0-lite", "upstreamModel": "wand-vega-video-1.0-lite",
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"model": "wand-vega-video-1.0-lite", "prompt": "几个角色在街头散步", "seconds": 5, "size": "720x1280",
			"metadata": map[string]any{"content": liteImages},
		}},
	})
	require.NoError(t, err)
	liteEncoded, err := common.Marshal(liteDecoded)
	require.NoError(t, err)
	var liteIntent map[string]any
	require.NoError(t, common.Unmarshal(liteEncoded, &liteIntent))
	liteCtx := map[string]any{
		"baseUrl": "https://vod.tencentcloudapi.com", "apiKey": "sid|sk|150001", "authHeader": "",
		"model": "wand-vega-video-1.0-lite", "upstreamModel": "wand-vega-video-1.0-lite",
		"requestBody": liteIntent["requestBody"], "upstream": map[string]any{"kind": "vendor"},
		"publicTaskId": "task_pub", "requestHeaders": map[string]any{}, "files": []any{},
	}
	liteUsage, err := plugin.Engine.Call(t.Context(), "extractUsage", liteCtx)
	require.NoError(t, err)
	liteUsageEncoded, err := common.Marshal(liteUsage)
	require.NoError(t, err)
	var liteFacts map[string]any
	require.NoError(t, common.Unmarshal(liteUsageEncoded, &liteFacts))
	assert.Equal(t, map[string]any{"seconds": float64(5), "resolution": "768P", "generate_audio": false, "image_count": float64(6)}, liteFacts)
	liteSubmit, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", liteCtx)
	require.NoError(t, err)
	liteSubmitEncoded, err := common.Marshal(liteSubmit)
	require.NoError(t, err)
	var liteDescriptor struct {
		Body string `json:"body"`
	}
	require.NoError(t, common.Unmarshal(liteSubmitEncoded, &liteDescriptor))
	assert.Contains(t, liteDescriptor.Body, `"ModelVersion":"1.0-lite"`)
	assert.Contains(t, liteDescriptor.Body, `"Resolution":"768P"`)

	liteCompleted, err := plugin.Engine.Call(t.Context(), "extractUsageOnComplete", map[string]any{"taskId": "tid", "model": "wand-vega-video-1.0-lite"}, map[string]any{}, map[string]any{
		"Response": map[string]any{
			"AigcVideoTask": map[string]any{
				"Status": "FINISH",
				"Output": map[string]any{
					"Usage": map[string]any{"InputImageCount": 0, "InputSeconds": 0, "OutputSeconds": 5, "TotalSeconds": 5},
					"FileInfos": []any{map[string]any{
						"FileUrl": "https://vod.example/lite.mp4", "UsageType": "",
						"MetaData": map[string]any{"Duration": 5.042, "Width": 768, "Height": 1344},
					}},
				},
			},
		},
	})
	require.NoError(t, err)
	liteCompletedEncoded, err := common.Marshal(liteCompleted)
	require.NoError(t, err)
	var liteCompletedFacts map[string]any
	require.NoError(t, common.Unmarshal(liteCompletedEncoded, &liteCompletedFacts))
	assert.Equal(t, map[string]any{"seconds": float64(5), "resolution": "768P"}, liteCompletedFacts)

	_, err = plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, map[string]any{
		"model": "wand-vega-video-1.0-lite", "upstreamModel": "wand-vega-video-1.0-lite",
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"model": "wand-vega-video-1.0-lite", "prompt": "她在微笑",
			"metadata": map[string]any{"content": []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": "asset://asset-1"}}}},
		}},
	})
	require.ErrorContains(t, err, "does not accept asset:// materials")

	liteRealPerson, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{
		"taskId": "tid", "model": "wand-vega-video-1.0-lite", "upstream": map[string]any{"kind": "vendor"},
		"apiKey": "sid|sk|150001", "baseUrl": "https://vod.tencentcloudapi.com",
	}, map[string]any{
		"Response": map[string]any{
			"Status": "FINISH",
			"AigcVideoTask": map[string]any{
				"Status":  "FINISH",
				"ErrCode": 70000,
				"Message": "task failed with status: FAIL, message: The input image may contain real person.",
				"Input": map[string]any{"FileInfos": []any{map[string]any{
					"Type": "Url", "Category": "Image", "Url": "https://example.com/person.jpg", "Usage": "Reference",
				}}},
				"Output": map[string]any{"FileInfos": []any{}},
			},
		},
	})
	require.NoError(t, err)
	liteRealPersonEncoded, err := common.Marshal(liteRealPerson)
	require.NoError(t, err)
	var liteRealPersonResult map[string]any
	require.NoError(t, common.Unmarshal(liteRealPersonEncoded, &liteRealPersonResult))
	assert.Equal(t, "FAILURE", liteRealPersonResult["status"])

	failedFinish, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{"taskId": "tid"}, map[string]any{
		"Response": map[string]any{
			"Status": "FINISH",
			"AigcVideoTask": map[string]any{
				"Status":  "FINISH",
				"ErrCode": 70000,
				"Message": "task failed with status: FAIL, message: The input may contain real person.",
				"Output":  map[string]any{"FileInfos": []any{}},
			},
		},
	})
	require.NoError(t, err)
	failedEncoded, err := common.Marshal(failedFinish)
	require.NoError(t, err)
	var failedResult map[string]any
	require.NoError(t, common.Unmarshal(failedEncoded, &failedResult))
	assert.Equal(t, "FAILURE", failedResult["status"])
	assert.Contains(t, failedResult["reason"], "may contain real person")

	portrait := "https://example.com/person.jpg"
	started, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{
		"taskId": "tid", "publicTaskId": "task_pub", "upstream": map[string]any{"kind": "vendor"},
		"apiKey": "sid|sk|150001", "baseUrl": "https://vod.tencentcloudapi.com",
	}, map[string]any{
		"Response": map[string]any{
			"Status": "FINISH",
			"AigcVideoTask": map[string]any{
				"Status":  "FINISH",
				"ErrCode": 70000,
				"Message": "task failed with status: FAIL, message: The input image may contain real person.",
				"Input": map[string]any{
					"ModelName": "wand-vega-video", "ModelVersion": "1.0-pro-mini", "Prompt": "她转头",
					"FileInfos":    []any{map[string]any{"Type": "Url", "Category": "Image", "Url": portrait, "Usage": "Reference"}},
					"OutputConfig": map[string]any{"Duration": 4, "Resolution": "720P", "StorageMode": "Temporary", "AudioGeneration": "Disabled"},
				},
				"Output": map[string]any{"FileInfos": []any{}},
			},
		},
	})
	require.NoError(t, err)
	startedEncoded, err := common.Marshal(started)
	require.NoError(t, err)
	var startedResult map[string]any
	require.NoError(t, common.Unmarshal(startedEncoded, &startedResult))
	assert.Equal(t, "IN_PROGRESS", startedResult["status"])
	startedState, _ := startedResult["state"].(map[string]any)
	require.NotNil(t, startedState)
	assert.Equal(t, "create_material", startedState["phase"])

	createMaterial, err := plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
		"taskId": "tid", "publicTaskId": "task_pub", "upstream": map[string]any{"kind": "vendor"},
		"apiKey": "sid|sk|150001", "baseUrl": "https://vod.tencentcloudapi.com", "state": startedState,
	})
	require.NoError(t, err)
	createEncoded, err := common.Marshal(createMaterial)
	require.NoError(t, err)
	var createDescriptor struct {
		Headers map[string]any `json:"headers"`
		Body    string         `json:"body"`
	}
	require.NoError(t, common.Unmarshal(createEncoded, &createDescriptor))
	assert.Equal(t, "CreateAigcMaterial", createDescriptor.Headers["X-TC-Action"])
	assert.Contains(t, createDescriptor.Body, `"AssetType":"Image"`)
	assert.Contains(t, createDescriptor.Body, `"IsRealPerson":"False"`)
	assert.Contains(t, createDescriptor.Body, portrait)

	waiting, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{
		"taskId": "tid", "state": startedState, "upstream": map[string]any{"kind": "vendor"},
	}, map[string]any{"Response": map[string]any{"TaskId": "material-1"}})
	require.NoError(t, err)
	waitingEncoded, err := common.Marshal(waiting)
	require.NoError(t, err)
	var waitingResult map[string]any
	require.NoError(t, common.Unmarshal(waitingEncoded, &waitingResult))
	waitingState, _ := waitingResult["state"].(map[string]any)
	require.NotNil(t, waitingState)
	assert.Equal(t, "wait_material", waitingState["phase"])
	assert.Equal(t, "material-1", waitingState["materialTaskId"])

	ready, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{
		"taskId": "tid", "state": waitingState, "upstream": map[string]any{"kind": "vendor"},
	}, map[string]any{
		"Response": map[string]any{
			"Status": "FINISH",
			"CreateAigcMaterialTask": map[string]any{
				"Status": "FINISH",
				"Output": map[string]any{"AssetId": "asset-person"},
			},
		},
	})
	require.NoError(t, err)
	readyEncoded, err := common.Marshal(ready)
	require.NoError(t, err)
	var readyResult map[string]any
	require.NoError(t, common.Unmarshal(readyEncoded, &readyResult))
	readyState, _ := readyResult["state"].(map[string]any)
	require.NotNil(t, readyState)
	assert.Equal(t, "retry_video", readyState["phase"])

	retryVideo, err := plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
		"taskId": "tid", "publicTaskId": "task_pub", "upstream": map[string]any{"kind": "vendor"},
		"apiKey": "sid|sk|150001", "baseUrl": "https://vod.tencentcloudapi.com", "state": readyState,
	})
	require.NoError(t, err)
	retryEncoded, err := common.Marshal(retryVideo)
	require.NoError(t, err)
	var retryDescriptor struct {
		Headers map[string]any `json:"headers"`
		Body    string         `json:"body"`
	}
	require.NoError(t, common.Unmarshal(retryEncoded, &retryDescriptor))
	assert.Equal(t, "CreateAigcVideoTask", retryDescriptor.Headers["X-TC-Action"])
	assert.Contains(t, retryDescriptor.Body, `"Url":"asset://asset-person"`)
	assert.Contains(t, retryDescriptor.Body, `"Duration":4`)
	assert.NotContains(t, retryDescriptor.Body, `"AspectRatio":"adaptive"`)

	decodedMaterial, err := plugin.Engine.CallPath(t.Context(), "native", []string{"createMaterial"}, map[string]any{
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"Url": "https://example.com/person.jpg", "AssetType": "Image",
		}},
	})
	require.NoError(t, err)
	materialEncoded, err := common.Marshal(decodedMaterial)
	require.NoError(t, err)
	var materialIntent map[string]any
	require.NoError(t, common.Unmarshal(materialEncoded, &materialIntent))
	assert.Equal(t, "submit", materialIntent["kind"])
	assert.Equal(t, "register_material", materialIntent["action"])
	assert.Equal(t, "wand-vega-material", materialIntent["model"])

	materialSubmit, err := plugin.Engine.Call(t.Context(), "buildSubmitRequest", map[string]any{
		"baseUrl": "https://vod.tencentcloudapi.com", "apiKey": "sid|sk|150001", "authHeader": "",
		"model": "wand-vega-material", "upstreamModel": "wand-vega-material", "action": "register_material",
		"requestBody": materialIntent["requestBody"], "upstream": map[string]any{"kind": "vendor"},
		"publicTaskId": "task_pub", "requestHeaders": map[string]any{}, "files": []any{},
	})
	require.NoError(t, err)
	materialSubmitEncoded, err := common.Marshal(materialSubmit)
	require.NoError(t, err)
	var materialDescriptor struct {
		Headers map[string]any `json:"headers"`
		Body    string         `json:"body"`
	}
	require.NoError(t, common.Unmarshal(materialSubmitEncoded, &materialDescriptor))
	assert.Equal(t, "CreateAigcMaterial", materialDescriptor.Headers["X-TC-Action"])
	assert.Contains(t, materialDescriptor.Body, `"AssetType":"Image"`)
	assert.Contains(t, materialDescriptor.Body, `"Url":"https://example.com/person.jpg"`)
	assert.Contains(t, materialDescriptor.Body, `"IsRealPerson":"False"`)

	usage, err := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
		"action": "register_material", "model": "wand-vega-material", "upstreamModel": "wand-vega-material",
		"requestBody": materialIntent["requestBody"],
	})
	require.NoError(t, err)
	usageEncoded, err := common.Marshal(usage)
	require.NoError(t, err)
	var usageFacts map[string]any
	require.NoError(t, common.Unmarshal(usageEncoded, &usageFacts))
	assert.Equal(t, float64(1), usageFacts["material_count"])

	materialDone, err := plugin.Engine.Call(t.Context(), "parseTaskResult", map[string]any{
		"taskId": "mid", "action": "register_material",
	}, map[string]any{
		"Response": map[string]any{
			"Status": "FINISH",
			"CreateAigcMaterialTask": map[string]any{
				"Status": "FINISH",
				"Output": map[string]any{"AssetId": "asset-public"},
			},
		},
	})
	require.NoError(t, err)
	materialDoneEncoded, err := common.Marshal(materialDone)
	require.NoError(t, err)
	var materialResult map[string]any
	require.NoError(t, common.Unmarshal(materialDoneEncoded, &materialResult))
	assert.Equal(t, "SUCCESS", materialResult["status"])
	assert.Equal(t, "asset://asset-public", materialResult["url"])
}
