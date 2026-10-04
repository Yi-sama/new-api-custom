package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
)

type VideoStudioParameterValues struct {
	Size          string
	AspectRatio   string
	Quality       string
	Count         *int
	Duration      *int
	Resolution    string
	Mode          string
	GenerateAudio *bool
	Watermark     *bool
	FPS           *int
}

func ValidateVideoStudioParameters(group, modelName, requestKind string, values VideoStudioParameterValues) error {
	profile, err := videoStudioParameterProfile(modelName, group, requestKind)
	if err != nil {
		return err
	}
	if err := validateVideoStudioStringParameter("size", values.Size, profile.Sizes); err != nil {
		return err
	}
	if err := validateVideoStudioStringParameter("aspect ratio", values.AspectRatio, profile.AspectRatios); err != nil {
		return err
	}
	if err := validateVideoStudioStringParameter("quality", values.Quality, profile.Qualities); err != nil {
		return err
	}
	if err := validateVideoStudioIntParameter("count", values.Count, profile.Counts); err != nil {
		return err
	}
	if err := validateVideoStudioIntParameter("duration", values.Duration, profile.Durations); err != nil {
		return err
	}
	if err := validateVideoStudioStringParameter("resolution", values.Resolution, profile.Resolutions); err != nil {
		return err
	}
	if err := validateVideoStudioStringParameter("mode", values.Mode, profile.Modes); err != nil {
		return err
	}
	if err := validateVideoStudioBoolParameter("generate_audio", values.GenerateAudio, profile.GenerateAudio); err != nil {
		return err
	}
	if err := validateVideoStudioBoolParameter("watermark", values.Watermark, profile.Watermarks); err != nil {
		return err
	}
	return validateVideoStudioIntParameter("fps", values.FPS, profile.FPS)
}

func videoStudioParameterProfile(modelName, group, requestKind string) (model.StudioParameterProfile, error) {
	if requestKind != "image" && requestKind != "video" {
		return model.StudioParameterProfile{}, fmt.Errorf("unsupported studio request kind %q", requestKind)
	}
	var metadata []model.Model
	if err := model.DB.Find(&metadata).Error; err != nil {
		return model.StudioParameterProfile{}, err
	}
	entry := videoStudioMatchingMetadata(metadata, modelName)
	if entry == nil {
		return model.StudioParameterProfile{}, nil
	}
	parameters, err := model.ParseStudioModelParameters(entry.Parameters)
	if err != nil {
		return model.StudioParameterProfile{}, fmt.Errorf("model %q has invalid parameter metadata: %w", entry.ModelName, err)
	}
	return parameters.Profile(requestKind, group), nil
}

func videoStudioMatchingMetadata(metadata []model.Model, modelName string) *model.Model {
	for i := range metadata {
		if metadata[i].NameRule == model.NameRuleExact && metadata[i].ModelName == modelName {
			return &metadata[i]
		}
	}
	for _, rule := range []int{model.NameRulePrefix, model.NameRuleSuffix, model.NameRuleContains} {
		for i := range metadata {
			entry := &metadata[i]
			if entry.NameRule != rule {
				continue
			}
			if (rule == model.NameRulePrefix && strings.HasPrefix(modelName, entry.ModelName)) ||
				(rule == model.NameRuleSuffix && strings.HasSuffix(modelName, entry.ModelName)) ||
				(rule == model.NameRuleContains && strings.Contains(modelName, entry.ModelName)) {
				return entry
			}
		}
	}
	return nil
}

func validateVideoStudioStringParameter(name, value string, allowed []string) error {
	if len(allowed) == 0 || strings.TrimSpace(value) == "" {
		return nil
	}
	for _, option := range allowed {
		if value == option {
			return nil
		}
	}
	return fmt.Errorf("%s is not available for this model", name)
}

func validateVideoStudioIntParameter(name string, value *int, allowed []int) error {
	if len(allowed) == 0 || value == nil {
		return nil
	}
	for _, option := range allowed {
		if *value == option {
			return nil
		}
	}
	return fmt.Errorf("%s is not available for this model", name)
}

func validateVideoStudioBoolParameter(name string, value *bool, allowed []bool) error {
	if len(allowed) == 0 || value == nil {
		return nil
	}
	for _, option := range allowed {
		if *value == option {
			return nil
		}
	}
	return fmt.Errorf("%s is not available for this model", name)
}
