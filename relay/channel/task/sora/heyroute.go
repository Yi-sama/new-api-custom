package sora

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const heyRouteBillingKey = "heyroute_video_billing"

type heyRouteBilling struct {
	Seconds    int
	Resolution string
	UnitPrice  float64
}

func (a *TaskAdaptor) isHeyRoute() bool {
	u, err := url.Parse(a.baseURL)
	return err == nil && (u.Hostname() == "heyroute.ai" || u.Hostname() == "cn.heyroute.ai")
}

func (a *TaskAdaptor) validateHeyRouteRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	fail := func(err error) *dto.TaskError {
		return service.TaskErrorWrapperLocal(err, "invalid_video_request", http.StatusBadRequest)
	}
	if info.Action == constant.TaskActionRemix {
		return fail(fmt.Errorf("HeyRoute does not support the Sora remix endpoint"))
	}
	fields := make(map[string]any)
	switch {
	case strings.HasPrefix(c.GetHeader("Content-Type"), "application/json"):
		if err := common.UnmarshalBodyReusable(c, &fields); err != nil {
			return fail(err)
		}
	case strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data"):
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return fail(err)
		}
		for name, values := range form.Value {
			if len(values) != 1 {
				return fail(fmt.Errorf("duplicate video field: %s", name))
			}
			fields[name] = values[0]
		}
	default:
		return fail(fmt.Errorf("video requests require JSON or multipart/form-data"))
	}
	modelName, _ := fields["model"].(string)
	prompt, _ := fields["prompt"].(string)
	if strings.TrimSpace(prompt) == "" {
		return fail(fmt.Errorf("prompt is required"))
	}

	minSeconds, maxSeconds, defaultSeconds := 4, 15, 4
	defaultResolution := ""
	switch modelName {
	case "grok-video":
		minSeconds, defaultSeconds = 6, 0
	case "grok-imagine-video", "grok-imagine-video-1.5":
		minSeconds, defaultSeconds, defaultResolution = 1, 8, "480p"
	case "minimax-h3-original-768p", "minimax-h3-original-1080p", "minimax-h3-original-cf-2k":
	case "seedance-2.5":
		maxSeconds, defaultResolution = 30, "720p"
	default:
		return fail(fmt.Errorf("HeyRoute video price is not configured for model %s", modelName))
	}
	seconds := defaultSeconds
	explicitSeconds := false
	for _, field := range []string{"seconds", "duration"} {
		value, exists := fields[field]
		if !exists {
			continue
		}
		n, err := strconv.Atoi(fmt.Sprint(value))
		if err != nil || n < minSeconds || n > maxSeconds || n > relaycommon.MaxTaskDurationSeconds {
			return fail(fmt.Errorf("%s must be an integer between %d and %d", field, minSeconds, maxSeconds))
		}
		if explicitSeconds && n != seconds {
			return fail(fmt.Errorf("seconds and duration must match"))
		}
		seconds, explicitSeconds = n, true
	}
	if modelName == "grok-video" && seconds != 6 && seconds != 10 && seconds != 15 {
		return fail(fmt.Errorf("grok-video requires explicit seconds of 6, 10 or 15"))
	}

	resolution := defaultResolution
	if value, exists := fields["resolution"]; exists {
		text, ok := value.(string)
		if !ok {
			return fail(fmt.Errorf("resolution must be a string"))
		}
		if text != "" {
			resolution = strings.ToLower(strings.TrimSpace(text))
		}
	}
	if defaultResolution != "" {
		if resolution != "480p" && resolution != "720p" && resolution != "1080p" {
			return fail(fmt.Errorf("unsupported video resolution"))
		}
		if modelName == "grok-imagine-video" && resolution == "1080p" {
			resolution = "720p"
		}
	}
	if operation, _ := fields["omni_reference_task_type"].(string); operation != "" {
		if modelName != "seedance-2.5" || operation != "extend" {
			// Editing uses the source clip duration, which cannot be reserved from this request.
			return fail(fmt.Errorf("only explicit-duration Seedance extend is supported; source-duration edits cannot be priced"))
		}
		if !explicitSeconds {
			return fail(fmt.Errorf("extend requires explicit seconds"))
		}
		resolution = "720p"
	}

	basePrice, baseConfigured := ratio_setting.GetModelPrice(modelName, false)
	price := basePrice
	configured := baseConfigured
	if defaultResolution != "" {
		price, configured = ratio_setting.GetModelPrice(modelName+"-"+resolution, false)
	}
	if !baseConfigured || basePrice <= 0 || math.IsInf(basePrice, 0) || math.IsNaN(basePrice) ||
		!configured || price <= 0 || math.IsInf(price, 0) || math.IsNaN(price) {
		return fail(fmt.Errorf("HeyRoute video price is not configured for %s at %s", modelName, resolution))
	}
	c.Set(heyRouteBillingKey, heyRouteBilling{Seconds: seconds, Resolution: resolution, UnitPrice: price})
	c.Set("task_request", relaycommon.TaskSubmitReq{
		Model: modelName, Prompt: prompt, Seconds: strconv.Itoa(seconds),
	})
	info.Action = constant.TaskActionTextGenerate
	return nil
}
