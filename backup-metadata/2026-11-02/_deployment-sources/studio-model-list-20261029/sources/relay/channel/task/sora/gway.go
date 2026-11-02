package sora

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const gwayBillingKey = "gway_video_billing"

type gwayVideoProfile struct {
	Currency        string   `json:"currency"`
	BillingUnit     string   `json:"billing_unit"`
	Durations       []int    `json:"durations"`
	Resolutions     []string `json:"resolutions"`
	RequireDuration bool     `json:"require_duration"`
}

type gwayVideoBilling struct {
	Fields           map[string]any
	Quantity         int
	UnitPrice        float64
	UploadReferences bool
}

func (a *TaskAdaptor) isGway() bool {
	u, err := url.Parse(a.baseURL)
	return err == nil && (u.Hostname() == "gway.dev" || u.Hostname() == "www.gway.dev")
}

func normalizeGwayVideo(fields map[string]any, profile gwayVideoProfile) (gwayVideoBilling, error) {
	fail := func(message string) (gwayVideoBilling, error) {
		return gwayVideoBilling{}, fmt.Errorf("%s", message)
	}
	if profile.Currency != "CNY" || (profile.BillingUnit != "second" && profile.BillingUnit != "request") ||
		len(profile.Durations) == 0 || len(profile.Resolutions) == 0 {
		return fail("Gway video billing profile is not configured")
	}
	if metadata, exists := fields["metadata"]; exists {
		values, ok := metadata.(map[string]any)
		if !ok {
			return fail("metadata must be an object")
		}
		for key, value := range values {
			if current, exists := fields[key]; exists && fmt.Sprint(current) != fmt.Sprint(value) {
				return fail("conflicting top-level and metadata video fields")
			}
			fields[key] = value
		}
		delete(fields, "metadata")
	}
	name, _ := fields["model"].(string)
	prompt, _ := fields["prompt"].(string)
	if name == "" || strings.TrimSpace(prompt) == "" {
		return fail("model and prompt are required")
	}
	if n, exists := fields["n"]; exists && fmt.Sprint(n) != "1" {
		return fail("Gway video requests support exactly one output")
	}
	seconds, explicit := profile.Durations[0], false
	for _, key := range []string{"seconds", "duration"} {
		if value, exists := fields[key]; exists {
			n, err := strconv.Atoi(fmt.Sprint(value))
			if err != nil || n < 1 || n > relaycommon.MaxTaskDurationSeconds || !slices.Contains(profile.Durations, n) {
				return fail("unsupported video duration")
			}
			if explicit && seconds != n {
				return fail("seconds and duration must match")
			}
			seconds, explicit = n, true
		}
	}
	if profile.RequireDuration && !explicit {
		return fail("this Gway model requires an explicit duration")
	}
	resolution := profile.Resolutions[0]
	if value, exists := fields["resolution"]; exists {
		text, ok := value.(string)
		if !ok {
			return fail("resolution must be a string")
		}
		if text != "" {
			resolution = strings.ToLower(strings.TrimSpace(text))
		}
	}
	if !slices.Contains(profile.Resolutions, resolution) {
		return fail("unsupported video resolution")
	}
	base, baseFound := ratio_setting.GetModelPrice(name, false)
	price, priceFound := ratio_setting.GetModelPrice(name+"-"+resolution, false)
	if !baseFound || !priceFound || base <= 0 || price <= 0 ||
		math.IsNaN(base) || math.IsInf(base, 0) || math.IsNaN(price) || math.IsInf(price, 0) {
		return fail("Gway video price is not configured")
	}
	fields["duration"], fields["resolution"] = seconds, resolution
	delete(fields, "seconds")
	if reference, exists := fields["input_reference"]; exists {
		if _, hasImages := fields["images"]; hasImages {
			return fail("use either input_reference or images")
		}
		if text, ok := reference.(string); ok {
			fields["images"] = []string{text}
		} else {
			fields["images"] = reference
		}
		delete(fields, "input_reference")
	}
	quantity := 1
	if profile.BillingUnit == "second" {
		quantity = seconds
	}
	return gwayVideoBilling{Fields: fields, Quantity: quantity, UnitPrice: price}, nil
}

func (a *TaskAdaptor) validateGwayRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	fail := func(err error) *dto.TaskError {
		return service.TaskErrorWrapperLocal(err, "invalid_gway_video_request", http.StatusBadRequest)
	}
	if info.Action == constant.TaskActionRemix {
		return fail(fmt.Errorf("Gway video remix is not supported"))
	}
	var fields map[string]any
	uploadReferences := false
	switch {
	case strings.HasPrefix(c.GetHeader("Content-Type"), "application/json"):
		if err := common.UnmarshalBodyReusable(c, &fields); err != nil {
			return fail(err)
		}
	case strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data"):
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return fail(fmt.Errorf("could not read video reference form: %w", err))
		}
		defer form.RemoveAll()
		fields = make(map[string]any, len(form.Value))
		for key, values := range form.Value {
			if len(values) != 1 {
				return fail(fmt.Errorf("video field %s must appear once", key))
			}
			fields[key] = values[0]
			switch key {
			case "metadata", "images", "watermark", "generate_audio", "camera_fixed", "return_last_frame":
				var value any
				if err := common.UnmarshalJsonStr(values[0], &value); err != nil {
					return fail(fmt.Errorf("video field %s must be valid JSON", key))
				}
				fields[key] = value
			}
		}
		for key, files := range form.File {
			if key != "input_reference" {
				return fail(fmt.Errorf("unsupported video file field: %s", key))
			}
			if len(files) > 20 {
				return fail(fmt.Errorf("at most 20 video reference images are supported"))
			}
			for _, file := range files {
				if file.Size <= 0 || file.Size > 32<<20 {
					return fail(fmt.Errorf("video reference images must be between 1 byte and 32 MiB"))
				}
			}
			uploadReferences = len(files) > 0
		}
	default:
		return fail(fmt.Errorf("Gway video generation requires JSON or multipart/form-data"))
	}
	name, _ := fields["model"].(string)
	var meta model.Model
	if err := model.DB.Where("model_name = ? AND status = ?", name, 1).First(&meta).Error; err != nil {
		return fail(fmt.Errorf("Gway model catalog is not configured"))
	}
	var parameters struct {
		Provider string           `json:"provider"`
		Video    gwayVideoProfile `json:"video"`
	}
	if err := common.UnmarshalJsonStr(meta.Parameters, &parameters); err != nil || parameters.Provider != "gway" {
		return fail(fmt.Errorf("Gway model billing profile is not configured"))
	}
	billing, err := normalizeGwayVideo(fields, parameters.Video)
	if err != nil {
		return fail(err)
	}
	if uploadReferences {
		if _, exists := billing.Fields["images"]; exists {
			return fail(fmt.Errorf("use either reference files or image URLs, not both"))
		}
	}
	billing.UploadReferences = uploadReferences
	c.Set(gwayBillingKey, billing)
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model: name, Prompt: fmt.Sprint(fields["prompt"]), Seconds: fmt.Sprint(fields["duration"]),
	})
	info.Action = constant.TaskActionTextGenerate
	return nil
}

func (a *TaskAdaptor) buildGwayRequestBody(c *gin.Context, info *relaycommon.RelayInfo, billing gwayVideoBilling) (io.Reader, error) {
	if billing.UploadReferences {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, fmt.Errorf("could not read video reference images: %w", err)
		}
		defer form.RemoveAll()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.WriteField("kind", "image"); err != nil {
			return nil, err
		}
		for _, header := range form.File["input_reference"] {
			file, err := header.Open()
			if err != nil {
				return nil, fmt.Errorf("could not open video reference image: %w", err)
			}
			part, err := writer.CreateFormFile("file", header.Filename)
			if err == nil {
				_, err = io.Copy(part, file)
			}
			file.Close()
			if err != nil {
				return nil, fmt.Errorf("could not prepare video reference image: %w", err)
			}
		}
		if err := writer.Close(); err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
			strings.TrimRight(a.baseURL, "/")+"/v1/media/upload", &body)
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", "Bearer "+a.apiKey)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		client, err := service.GetHttpClientWithProxySettings(info.ChannelSetting.Proxy, info.ChannelSetting)
		if err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("Gway reference image upload failed; video was not submitted: %w", err)
		}
		defer response.Body.Close()
		var result struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
			Data    struct {
				Items []struct {
					URL string `json:"url"`
				} `json:"items"`
			} `json:"data"`
		}
		if err := common.DecodeJson(io.LimitReader(response.Body, 65536), &result); err != nil {
			return nil, fmt.Errorf("Gway reference image upload returned an invalid response (HTTP %d); video was not submitted", response.StatusCode)
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 || !result.Success {
			message := strings.ReplaceAll(result.Message, a.apiKey, "[redacted]")
			return nil, fmt.Errorf("Gway reference image upload failed (HTTP %d): %s; video was not submitted",
				response.StatusCode, common.MaskSensitiveInfo(message))
		}
		if len(result.Data.Items) != len(form.File["input_reference"]) {
			return nil, fmt.Errorf("Gway reference image upload returned an incomplete image list; video was not submitted")
		}
		images := make([]string, 0, len(result.Data.Items))
		for _, item := range result.Data.Items {
			parsed, err := url.Parse(item.URL)
			if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
				return nil, fmt.Errorf("Gway reference image upload returned an invalid image URL; video was not submitted")
			}
			images = append(images, item.URL)
		}
		billing.Fields["images"] = images
	}
	return buildGwayVideoBody(billing, info.UpstreamModelName)
}

func buildGwayVideoBody(billing gwayVideoBilling, upstreamModel string) (io.Reader, error) {
	billing.Fields["model"] = upstreamModel
	data, err := common.Marshal(billing.Fields)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}
