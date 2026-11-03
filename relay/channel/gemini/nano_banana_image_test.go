package gemini

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNanoBananaImagesConversion(t *testing.T) {
	for _, model := range []string{"nano-banana-2", "nano-banana-2.1", "nano-banana-pro"} {
		t.Run(model, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayMode: constant.RelayModeImagesGenerations,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: model,
					ChannelBaseUrl:    "https://api.uuyue.top",
				},
			}
			adaptor := &Adaptor{}
			converted, err := adaptor.ConvertImageRequest(nil, info, dto.ImageRequest{
				Model: model, Prompt: "A red cup", Size: "1024x1024",
				N: common.GetPointer(uint(1)), ResponseFormat: "b64_json",
			})
			require.NoError(t, err)
			request, ok := converted.(dto.GeminiChatRequest)
			require.True(t, ok)
			require.Len(t, request.Contents, 1)
			assert.Equal(t, "user", request.Contents[0].Role)
			require.Len(t, request.Contents[0].Parts, 1)
			assert.Equal(t, "A red cup", request.Contents[0].Parts[0].Text)
			assert.Equal(t, []string{"TEXT", "IMAGE"}, request.GenerationConfig.ResponseModalities)
			assert.JSONEq(t, `{"aspectRatio":"1:1","imageSize":"1K"}`, string(request.GenerationConfig.ImageConfig))
			url, err := adaptor.GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, "https://api.uuyue.top/v1beta/models/"+model+":generateContent", url)
			header := http.Header{}
			info.ApiKey = "test-key"
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
			assert.Equal(t, "test-key", header.Get("x-goog-api-key"))
		})
	}
}

func TestNanoBananaImagesParameters(t *testing.T) {
	for _, test := range []struct {
		name    string
		request dto.ImageRequest
		config  string
		mode    int
		wantErr string
	}{
		{name: "portrait", request: dto.ImageRequest{Size: "1024x1792"}, config: `{"aspectRatio":"9:16","imageSize":"1K"}`},
		{name: "aspect ratio", request: dto.ImageRequest{Size: "4:3"}, config: `{"aspectRatio":"4:3","imageSize":"1K"}`},
		{name: "high quality", request: dto.ImageRequest{Quality: "high"}, config: `{"aspectRatio":"1:1","imageSize":"2K"}`},
		{name: "resolution", request: dto.ImageRequest{Size: "2048x2048"}, config: `{"aspectRatio":"1:1","imageSize":"2K"}`},
		{name: "multi image", request: dto.ImageRequest{N: common.GetPointer(uint(2))}, wantErr: "n=1"},
		{name: "stream", request: dto.ImageRequest{Stream: common.GetPointer(true)}, wantErr: "streaming"},
		{name: "format", request: dto.ImageRequest{ResponseFormat: "file"}, wantErr: "response_format"},
		{name: "size", request: dto.ImageRequest{Size: "invalid"}, wantErr: "size"},
		{name: "reference", request: dto.ImageRequest{Image: []byte(`"https://example.com/a.png"`)}, wantErr: "editing"},
		{name: "multipart edit", mode: constant.RelayModeImagesEdits, wantErr: "editing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				RelayMode: test.mode,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: "nano-banana-pro",
				},
			}
			converted, err := (&Adaptor{}).ConvertImageRequest(nil, info, test.request)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			request := converted.(dto.GeminiChatRequest)
			assert.JSONEq(t, test.config, string(request.GenerationConfig.ImageConfig))
		})
	}
}

func TestImagenImagesConversionUnchanged(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "imagen-4.0-generate-001"}}
	converted, err := (&Adaptor{}).ConvertImageRequest(nil, info, dto.ImageRequest{
		Prompt: "A red cup", N: common.GetPointer(uint(2)), Size: "1536x1024", Quality: "high",
	})
	require.NoError(t, err)
	request, ok := converted.(dto.GeminiImageRequest)
	require.True(t, ok)
	assert.Equal(t, "A red cup", request.Instances[0].Prompt)
	assert.Equal(t, 2, request.Parameters.SampleCount)
	assert.Equal(t, "3:2", request.Parameters.AspectRatio)
	assert.Equal(t, "2K", request.Parameters.ImageSize)
}

func TestNanoBananaImagesResponse(t *testing.T) {
	for _, format := range []string{"", "b64_json", "url"} {
		t.Run(format, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			info := &relaycommon.RelayInfo{
				RelayMode: constant.RelayModeImagesGenerations,
				Request:   &dto.ImageRequest{ResponseFormat: format},
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: "nano-banana-pro",
				},
			}
			body := `{"candidates":[{"content":{"parts":[
				{"text":"Here is the cup"},
				{"thought":true,"inlineData":{"mimeType":"image/png","data":"dGhvdWdodA=="}},
				{"inlineData":{"mimeType":"audio/wav","data":"YXVkaW8="}},
				{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}
			]}}],"promptFeedback":{"blockReason":""},"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":1400,"totalTokenCount":1420}}`
			usageValue, apiErr := (&Adaptor{}).DoResponse(c, &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
			}, info)
			require.Nil(t, apiErr)
			usage, ok := usageValue.(*dto.Usage)
			require.True(t, ok)
			assert.Equal(t, 20, usage.PromptTokens)
			assert.Equal(t, 1400, usage.CompletionTokens)
			assert.Equal(t, 1420, usage.TotalTokens)
			require.NotNil(t, usage.BillingUsage)
			assert.Equal(t, 200, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			var result dto.ImageResponse
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &result))
			require.Len(t, result.Data, 1)
			assert.Positive(t, result.Created)
			if format == "url" {
				assert.Equal(t, "data:image/png;base64,aW1hZ2U=", result.Data[0].Url)
				assert.Empty(t, result.Data[0].B64Json)
			} else {
				assert.Equal(t, "aW1hZ2U=", result.Data[0].B64Json)
				assert.Empty(t, result.Data[0].Url)
			}
			assert.NotContains(t, recorder.Body.String(), `"choices"`)
		})
	}
}

func TestNanoBananaImagesRejectsEmptyOrBlockedResponse(t *testing.T) {
	for _, test := range []struct {
		name   string
		body   string
		status int
	}{
		{name: "text only", body: `{"candidates":[{"content":{"parts":[{"text":"no image"}]}}]}`, status: 502},
		{name: "empty data", body: `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":""}}]}}]}`, status: 502},
		{name: "blocked", body: `{"promptFeedback":{"blockReason":"SAFETY"}}`, status: 400},
		{name: "invalid json", body: `not json`, status: 502},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			info := &relaycommon.RelayInfo{
				Request:     &dto.ImageRequest{},
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "nano-banana-pro"},
			}
			usage, apiErr := GeminiNanoBananaImageHandler(c, info, &http.Response{
				StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body)),
			})
			require.NotNil(t, apiErr)
			assert.Equal(t, test.status, apiErr.StatusCode)
			assert.Nil(t, usage)
			assert.Empty(t, recorder.Body.String())
		})
	}
}

func TestNanoBananaImagesMissingCompletionUsage(t *testing.T) {
	for _, metadata := range []string{"", `,"usageMetadata":{"promptTokenCount":20,"totalTokenCount":20}`} {
		t.Run(metadata, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			info := &relaycommon.RelayInfo{
				Request:     &dto.ImageRequest{},
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "nano-banana-pro"},
			}
			info.SetEstimatePromptTokens(20)
			body := `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}}]` + metadata + `}`
			usage, apiErr := GeminiNanoBananaImageHandler(c, info, &http.Response{
				StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)),
			})
			require.Nil(t, apiErr)
			require.NotNil(t, usage)
			assert.Equal(t, 20, usage.PromptTokens)
			assert.Equal(t, 1400, usage.CompletionTokens)
			assert.Equal(t, 1420, usage.TotalTokens)
			require.NotNil(t, usage.BillingUsage)
			assert.True(t, usage.BillingUsage.Estimated)
		})
	}
}
