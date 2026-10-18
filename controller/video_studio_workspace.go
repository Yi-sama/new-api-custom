package controller

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type videoStudioWorkspaceRequest struct {
	BaseRevision int64           `json:"baseRevision"`
	Manifest     json.RawMessage `json:"manifest"`
}

func videoStudioWorkspaceError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"success": false, "code": code, "message": message})
}

func videoStudioWorkspaceResponse(c *gin.Context, revision int64, manifest string) {
	data := gin.H{"revision": revision, "manifest": nil}
	if manifest != "" {
		data["manifest"] = json.RawMessage(manifest)
	}
	common.ApiSuccess(c, data)
}

func GetVideoStudioWorkspace(c *gin.Context) {
	workspace, err := model.FindStudioWorkspace(c.GetInt("id"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		videoStudioWorkspaceResponse(c, 0, "")
		return
	}
	if err != nil {
		videoStudioWorkspaceError(c, http.StatusInternalServerError, "WORKSPACE_READ_FAILED", "could not load workspace")
		return
	}
	videoStudioWorkspaceResponse(c, workspace.Revision, workspace.Manifest)
}

func PutVideoStudioWorkspace(c *gin.Context) {
	defer common.CleanupBodyStorage(c)
	if !strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		videoStudioWorkspaceError(c, http.StatusBadRequest, "WORKSPACE_INVALID", "workspace must be JSON")
		return
	}
	var request videoStudioWorkspaceRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		status := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) {
			status = http.StatusRequestEntityTooLarge
		}
		videoStudioWorkspaceError(c, status, "WORKSPACE_INVALID", "invalid workspace manifest")
		return
	}
	if request.BaseRevision < 0 || len(request.Manifest) == 0 || string(request.Manifest) == "null" {
		videoStudioWorkspaceError(c, http.StatusBadRequest, "WORKSPACE_INVALID", "workspace manifest and base revision are required")
		return
	}
	storageKeys, err := validateVideoStudioWorkspaceManifest(request.Manifest)
	if err != nil {
		videoStudioWorkspaceError(c, http.StatusBadRequest, "WORKSPACE_INVALID", err.Error())
		return
	}
	for key := range storageKeys {
		keyDigest := sha256.Sum256([]byte(key))
		if _, err := model.FindStudioWorkspaceFile(c.GetInt("id"), hex.EncodeToString(keyDigest[:])); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				videoStudioWorkspaceError(c, http.StatusBadRequest, "WORKSPACE_INVALID", "workspace references a file that has not been uploaded")
				return
			}
			videoStudioWorkspaceError(c, http.StatusInternalServerError, "WORKSPACE_READ_FAILED", "could not validate workspace files")
			return
		}
	}
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(request.Manifest, &fields); err != nil {
		videoStudioWorkspaceError(c, http.StatusBadRequest, "WORKSPACE_INVALID", "workspace manifest is invalid")
		return
	}
	if _, hasSkills := fields["promptSkills"]; !hasSkills && request.BaseRevision > 0 {
		previous, err := model.FindStudioWorkspace(c.GetInt("id"))
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			videoStudioWorkspaceError(c, http.StatusInternalServerError, "WORKSPACE_READ_FAILED", "could not load workspace")
			return
		}
		if err == nil && previous.Revision == request.BaseRevision {
			var previousFields map[string]json.RawMessage
			if err := common.UnmarshalJsonStr(previous.Manifest, &previousFields); err != nil {
				videoStudioWorkspaceError(c, http.StatusInternalServerError, "WORKSPACE_READ_FAILED", "could not read saved workspace")
				return
			}
			// Older open pages cannot delete Skills they do not understand.
			if skills, exists := previousFields["promptSkills"]; exists {
				fields["promptSkills"] = skills
				request.Manifest, err = common.Marshal(fields)
				if err != nil {
					videoStudioWorkspaceError(c, http.StatusInternalServerError, "WORKSPACE_WRITE_FAILED", "could not save workspace")
					return
				}
			}
		}
	}
	workspace, saved, err := model.SaveStudioWorkspace(c.GetInt("id"), request.BaseRevision, string(request.Manifest))
	if err != nil {
		videoStudioWorkspaceError(c, http.StatusInternalServerError, "WORKSPACE_WRITE_FAILED", "could not save workspace")
		return
	}
	if !saved {
		videoStudioWorkspaceError(c, http.StatusConflict, "WORKSPACE_CONFLICT", "workspace has changed")
		return
	}
	videoStudioWorkspaceResponse(c, workspace.Revision, workspace.Manifest)
}

func validateVideoStudioWorkspaceManifest(raw json.RawMessage) (map[string]struct{}, error) {
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, errors.New("workspace manifest must be an object")
	}
	required := []string{"version", "canvas", "assets", "imageLogs", "videoLogs"}
	allowed := map[string]struct{}{"version": {}, "canvas": {}, "assets": {}, "imageLogs": {}, "videoLogs": {}, "promptSkills": {}}
	for name := range fields {
		if _, ok := allowed[name]; !ok {
			return nil, errors.New("workspace manifest contains an unsupported field")
		}
	}
	for _, name := range required {
		if _, exists := fields[name]; !exists {
			return nil, errors.New("workspace manifest is incomplete")
		}
	}
	var version int
	if err := common.Unmarshal(fields["version"], &version); err != nil || version != 1 {
		return nil, errors.New("workspace manifest version is unsupported")
	}
	if err := validateVideoStudioWorkspaceObject(fields["canvas"], "projects", "deletedProjects"); err != nil {
		return nil, err
	}
	if err := validateVideoStudioWorkspaceObject(fields["assets"], "assets"); err != nil {
		return nil, err
	}
	for _, name := range []string{"imageLogs", "videoLogs"} {
		var items []any
		if err := common.Unmarshal(fields[name], &items); err != nil || items == nil {
			return nil, errors.New("workspace logs must be arrays")
		}
	}
	if rawSkills, exists := fields["promptSkills"]; exists {
		var skills []struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			Instructions string `json:"instructions"`
		}
		if err := common.Unmarshal(rawSkills, &skills); err != nil || skills == nil {
			return nil, errors.New("workspace promptSkills must be an array of Skills")
		}
		for _, skill := range skills {
			if strings.TrimSpace(skill.ID) == "" || strings.TrimSpace(skill.Name) == "" || strings.TrimSpace(skill.Instructions) == "" {
				return nil, errors.New("workspace promptSkills require id, name and instructions")
			}
		}
	}
	var manifest any
	if err := common.Unmarshal(raw, &manifest); err != nil {
		return nil, errors.New("workspace manifest is invalid")
	}
	keys := make(map[string]struct{})
	collectVideoStudioWorkspaceStorageKeys(manifest, keys)
	return keys, nil
}

func validateVideoStudioWorkspaceObject(raw json.RawMessage, allowedFields ...string) error {
	if string(raw) == "null" {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil || fields == nil {
		return errors.New("workspace container must be an object or null")
	}
	allowed := make(map[string]struct{}, len(allowedFields))
	for _, field := range allowedFields {
		allowed[field] = struct{}{}
	}
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			return errors.New("workspace container contains an unsupported field")
		}
	}
	return nil
}

func collectVideoStudioWorkspaceStorageKeys(value any, keys map[string]struct{}) {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			collectVideoStudioWorkspaceStorageKeys(item, keys)
		}
	case map[string]any:
		for name, item := range typed {
			if name == "storageKey" {
				if key, ok := item.(string); ok && key != "" {
					keys[key] = struct{}{}
				}
			}
			collectVideoStudioWorkspaceStorageKeys(item, keys)
		}
	}
}

func PutVideoStudioWorkspaceFile(c *gin.Context) {
	defer common.CleanupBodyStorage(c)
	key := c.Param("key")
	contentHash := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Content-SHA256")))
	if key == "" || len(contentHash) != sha256.Size*2 {
		videoStudioWorkspaceError(c, http.StatusBadRequest, "FILE_INVALID", "file key and X-Content-SHA256 are required")
		return
	}
	if _, err := hex.DecodeString(contentHash); err != nil {
		videoStudioWorkspaceError(c, http.StatusBadRequest, "FILE_INVALID", "X-Content-SHA256 must be a SHA-256 hash")
		return
	}
	body, err := common.GetBodyStorage(c)
	if err != nil {
		status := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) {
			status = http.StatusRequestEntityTooLarge
		}
		videoStudioWorkspaceError(c, status, "FILE_INVALID", "could not read file")
		return
	}
	reader, err := body.NewReader()
	if err != nil {
		videoStudioWorkspaceError(c, http.StatusInternalServerError, "FILE_WRITE_FAILED", "could not store file")
		return
	}
	defer reader.Close()
	userID := c.GetInt("id")
	if err := service.StoreVideoStudioWorkspaceBlob(userID, contentHash, reader); err != nil {
		if strings.Contains(err.Error(), "X-Content-SHA256") {
			videoStudioWorkspaceError(c, http.StatusBadRequest, "FILE_INVALID", err.Error())
			return
		}
		videoStudioWorkspaceError(c, http.StatusInternalServerError, "FILE_WRITE_FAILED", "could not store file")
		return
	}
	keyDigest := sha256.Sum256([]byte(key))
	contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	file, created, err := model.CreateStudioWorkspaceFile(&model.StudioWorkspaceFile{
		UserID: userID, KeyHash: hex.EncodeToString(keyDigest[:]), ContentHash: contentHash, ContentType: contentType, CreatedAt: common.GetTimestamp(),
	})
	if err != nil {
		videoStudioWorkspaceError(c, http.StatusInternalServerError, "FILE_WRITE_FAILED", "could not save file metadata")
		return
	}
	if !created && file.ContentHash != contentHash {
		videoStudioWorkspaceError(c, http.StatusConflict, "FILE_CONFLICT", "file key already belongs to different content")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"key": key, "sha256": file.ContentHash}})
}

func GetVideoStudioWorkspaceFile(c *gin.Context) {
	keyDigest := sha256.Sum256([]byte(c.Param("key")))
	file, err := model.FindStudioWorkspaceFile(c.GetInt("id"), hex.EncodeToString(keyDigest[:]))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		videoStudioWorkspaceError(c, http.StatusInternalServerError, "FILE_READ_FAILED", "could not load file")
		return
	}
	path := service.VideoStudioWorkspaceBlobPath(c.GetInt("id"), file.ContentHash)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return
		}
		videoStudioWorkspaceError(c, http.StatusInternalServerError, "FILE_READ_FAILED", "could not load file")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Content-SHA256", file.ContentHash)
	c.Header("Content-Type", file.ContentType)
	c.File(path)
}
