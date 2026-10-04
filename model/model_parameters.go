package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type StudioParameterProfile struct {
	Sizes         []string `json:"sizes,omitempty"`
	AspectRatios  []string `json:"aspect_ratios,omitempty"`
	Qualities     []string `json:"qualities,omitempty"`
	Counts        []int    `json:"counts,omitempty"`
	Durations     []int    `json:"durations,omitempty"`
	Resolutions   []string `json:"resolutions,omitempty"`
	Modes         []string `json:"modes,omitempty"`
	GenerateAudio []bool   `json:"generate_audio,omitempty"`
	Watermarks    []bool   `json:"watermarks,omitempty"`
	FPS           []int    `json:"fps,omitempty"`
}

type StudioModelParameters struct {
	Image  StudioParameterProfile             `json:"image,omitempty"`
	Video  StudioParameterProfile             `json:"video,omitempty"`
	Groups map[string]StudioParameterOverride `json:"groups,omitempty"`
}

type StudioParameterOverride struct {
	Image StudioParameterProfile `json:"image,omitempty"`
	Video StudioParameterProfile `json:"video,omitempty"`
}

func ParseStudioModelParameters(raw string) (StudioModelParameters, error) {
	if strings.TrimSpace(raw) == "" {
		return StudioModelParameters{}, nil
	}
	var parameters StudioModelParameters
	if err := common.Unmarshal([]byte(raw), &parameters); err != nil {
		return StudioModelParameters{}, fmt.Errorf("invalid model parameters: %w", err)
	}
	if err := validateStudioParameterProfile(parameters.Image); err != nil {
		return StudioModelParameters{}, fmt.Errorf("invalid image parameters: %w", err)
	}
	if err := validateStudioParameterProfile(parameters.Video); err != nil {
		return StudioModelParameters{}, fmt.Errorf("invalid video parameters: %w", err)
	}
	for group, override := range parameters.Groups {
		if strings.TrimSpace(group) == "" {
			return StudioModelParameters{}, fmt.Errorf("group override name is required")
		}
		if err := validateStudioParameterProfile(override.Image); err != nil {
			return StudioModelParameters{}, fmt.Errorf("invalid image parameters for group %q: %w", group, err)
		}
		if err := validateStudioParameterProfile(override.Video); err != nil {
			return StudioModelParameters{}, fmt.Errorf("invalid video parameters for group %q: %w", group, err)
		}
	}
	return parameters, nil
}

func (parameters StudioModelParameters) Profile(kind, group string) StudioParameterProfile {
	var profile StudioParameterProfile
	if kind == "image" {
		profile = parameters.Image
	} else if kind == "video" {
		profile = parameters.Video
	}
	override, ok := parameters.Groups[group]
	if !ok {
		return profile
	}
	if kind == "image" {
		return mergeStudioParameterProfile(profile, override.Image)
	}
	if kind == "video" {
		return mergeStudioParameterProfile(profile, override.Video)
	}
	return StudioParameterProfile{}
}

func validateStudioParameterProfile(profile StudioParameterProfile) error {
	for _, values := range [][]string{profile.Sizes, profile.AspectRatios, profile.Qualities, profile.Resolutions, profile.Modes} {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("string values must not be empty")
			}
		}
	}
	for _, values := range [][]int{profile.Counts, profile.Durations, profile.FPS} {
		for _, value := range values {
			if value <= 0 {
				return fmt.Errorf("numeric values must be positive")
			}
		}
	}
	return nil
}

func mergeStudioParameterProfile(base, override StudioParameterProfile) StudioParameterProfile {
	if len(override.Sizes) > 0 {
		base.Sizes = override.Sizes
	}
	if len(override.AspectRatios) > 0 {
		base.AspectRatios = override.AspectRatios
	}
	if len(override.Qualities) > 0 {
		base.Qualities = override.Qualities
	}
	if len(override.Counts) > 0 {
		base.Counts = override.Counts
	}
	if len(override.Durations) > 0 {
		base.Durations = override.Durations
	}
	if len(override.Resolutions) > 0 {
		base.Resolutions = override.Resolutions
	}
	if len(override.Modes) > 0 {
		base.Modes = override.Modes
	}
	if len(override.GenerateAudio) > 0 {
		base.GenerateAudio = override.GenerateAudio
	}
	if len(override.Watermarks) > 0 {
		base.Watermarks = override.Watermarks
	}
	if len(override.FPS) > 0 {
		base.FPS = override.FPS
	}
	return base
}
