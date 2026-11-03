package controller

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

func studioOCMJBody(c *gin.Context, fields map[string]any, modelName string) (io.Reader, string, string, error) {
	if _, exists := fields["mask"]; exists {
		return nil, "", "", errors.New("MJ does not support masks")
	}
	if _, exists := fields["action"]; exists {
		return nil, "", "", errors.New("studio MJ jobs only support initial image generation")
	}
	if n, exists := fields["n"]; exists && fmt.Sprint(n) != "1" {
		return nil, "", "", errors.New("MJ requires n=1 (one task returns four images)")
	}
	prompt, _ := fields["prompt"].(string)
	if strings.TrimSpace(prompt) == "" || utf8.RuneCountInString(prompt) > 32768 {
		return nil, "", "", errors.New("MJ prompt must contain 1 to 32768 characters")
	}
	payload := map[string]any{"model": modelName, "prompt": prompt, "n": 1, "stream": false}
	stable := strings.HasPrefix(strings.ToLower(modelName), "mj-")
	for _, key := range []string{"quality", "response_format", "hd"} {
		if value, exists := fields[key]; exists {
			payload[key] = value
		}
	}
	if stable {
		for _, key := range []string{"raw", "tile", "negative_prompt", "seed", "stylize", "chaos", "weird", "iw", "cw", "sw", "dw", "cref", "sref", "dref"} {
			if value, exists := fields[key]; exists {
				payload[key] = value
			}
		}
	} else if fields["tile"] == true || fields["tile"] == "true" {
		return nil, "", "", errors.New("this MJ model does not support tile")
	}
	for _, key := range []string{"hd", "raw", "tile"} {
		if value, exists := payload[key]; exists {
			boolean, err := strconv.ParseBool(fmt.Sprint(value))
			if err != nil {
				return nil, "", "", fmt.Errorf("MJ %s must be a boolean", key)
			}
			payload[key] = boolean
		}
	}
	if quality, exists := payload["quality"]; exists {
		switch quality {
		case "0.25", "0.5", "1", "2":
		default:
			return nil, "", "", errors.New("MJ quality must be 0.25, 0.5, 1 or 2")
		}
	}
	if format, exists := payload["response_format"]; exists && format != "url" && format != "b64_json" {
		return nil, "", "", errors.New("MJ response_format must be url or b64_json")
	}
	size, _ := fields["size"].(string)
	ratio, _ := fields["aspect_ratio"].(string)
	if size == "1K" || size == "2K" {
		if size == "2K" {
			if payload["hd"] == false {
				return nil, "", "", errors.New("MJ size=2K conflicts with hd=false")
			}
			payload["hd"] = true
		}
		size = ""
	}
	if ratio != "" {
		if size != "" && size != ratio {
			return nil, "", "", errors.New("conflicting MJ aspect ratios")
		}
		size = ratio
	}
	if size != "" {
		switch size {
		case "1:1", "3:2", "2:3", "4:3", "3:4", "5:4", "4:5", "16:9", "9:16", "21:9":
		case "auto", "9:21":
			if !stable {
				return nil, "", "", errors.New("this MJ model does not support the selected aspect ratio")
			}
		default:
			return nil, "", "", errors.New("unsupported MJ aspect ratio")
		}
		payload["size"] = size
	}
	if payload["hd"] == true && !strings.Contains(modelName, "v8.1") && !strings.Contains(modelName, "v8.2") {
		return nil, "", "", errors.New("MJ HD output requires v8.1 or v8.2")
	}

	references := []any{}
	if image, exists := fields["image"]; exists {
		references = append(references, image)
	}
	if images, exists := fields["images"]; exists {
		list, ok := images.([]any)
		if !ok {
			return nil, "", "", errors.New("MJ images must be an array")
		}
		references = append(references, list...)
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		form, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, "", "", err
		}
		defer form.RemoveAll()
		for key, files := range form.File {
			if key != "image" && key != "image[]" {
				return nil, "", "", errors.New("MJ only accepts reference image files")
			}
			for _, header := range files {
				file, err := header.Open()
				if err != nil {
					return nil, "", "", err
				}
				limit := int64(constant.MaxFileDownloadMB) * 1024 * 1024
				data, err := io.ReadAll(io.LimitReader(file, limit+1))
				file.Close()
				if err != nil {
					return nil, "", "", err
				}
				mime := http.DetectContentType(data)
				if int64(len(data)) > limit || !strings.HasPrefix(mime, "image/") {
					return nil, "", "", errors.New("invalid MJ reference image or download size limit exceeded")
				}
				references = append(references, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(data))
			}
		}
	}
	maxReferences := 1
	if stable {
		maxReferences = 5
	}
	if len(references) > maxReferences {
		return nil, "", "", fmt.Errorf("this MJ model supports at most %d reference images", maxReferences)
	}
	for _, reference := range references {
		image, ok := reference.(string)
		if !ok {
			return nil, "", "", errors.New("MJ reference must be an image URL or Data URI")
		}
		if strings.HasPrefix(image, "data:image/") {
			header, _, found := strings.Cut(image, ",")
			if !found || !strings.HasSuffix(header, ";base64") {
				return nil, "", "", errors.New("invalid MJ image Data URI")
			}
		} else {
			u, err := url.Parse(image)
			if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, "", "", errors.New("MJ reference must be an image URL or Data URI")
			}
		}
	}
	if len(references) == 1 {
		payload["image"] = references[0]
	} else if len(references) > 1 {
		payload["images"] = references
	}
	body, err := common.Marshal(payload)
	return bytes.NewReader(body), "application/json", "/images/midjourney/generations", err
}
