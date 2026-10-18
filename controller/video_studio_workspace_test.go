package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupVideoStudioWorkspaceTest(t *testing.T) (*gin.Engine, string, service.AuthIdentity, *model.UserSession) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("VIDEO_STUDIO_DIST_DIR", t.TempDir())
	t.Setenv("VIDEO_STUDIO_DATA_DIR", t.TempDir())
	identity, session := setupImageStudioSession(t, common.RoleCommonUser)
	require.NoError(t, model.DB.AutoMigrate(&model.StudioWorkspace{}, &model.StudioWorkspaceFile{}))
	token, _, err := service.IssueVideoStudioToken(identity, time.Unix(session.ExpiresAt, 0))
	require.NoError(t, err)
	router := gin.New()
	router.Use(VideoStudioBridgeAuth())
	router.GET("/storage/workspace", GetVideoStudioWorkspace)
	router.PUT("/storage/workspace", PutVideoStudioWorkspace)
	router.GET("/storage/files/:key", GetVideoStudioWorkspaceFile)
	router.PUT("/storage/files/:key", PutVideoStudioWorkspaceFile)
	return router, token, identity, session
}

func executeVideoStudioWorkspaceRequest(t *testing.T, router *gin.Engine, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestVideoStudioWorkspaceSnapshotAndFiles(t *testing.T) {
	router, token, _, _ := setupVideoStudioWorkspaceTest(t)

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/storage/workspace", nil))
	assert.Equal(t, http.StatusUnauthorized, unauthenticated.Code)

	empty := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/workspace", "")
	require.Equal(t, http.StatusOK, empty.Code)
	var emptyData struct {
		Data struct {
			Revision int             `json:"revision"`
			Manifest json.RawMessage `json:"manifest"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(empty.Body.Bytes(), &emptyData))
	assert.Equal(t, 0, emptyData.Data.Revision)
	assert.Equal(t, "null", string(emptyData.Data.Manifest))

	manifest := `{"version":1,"canvas":{"projects":[],"deletedProjects":[]},"assets":{"assets":[]},"imageLogs":[],"videoLogs":[]}`
	created := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":0,"manifest":`+manifest+`}`)
	require.Equal(t, http.StatusOK, created.Code)
	var createdData struct {
		Data struct {
			Revision int             `json:"revision"`
			Manifest json.RawMessage `json:"manifest"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(created.Body.Bytes(), &createdData))
	assert.Equal(t, 1, createdData.Data.Revision)
	assert.Equal(t, manifest, string(createdData.Data.Manifest))

	persisted := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/workspace", "")
	require.Equal(t, http.StatusOK, persisted.Code)
	var persistedData struct {
		Data struct {
			Revision int             `json:"revision"`
			Manifest json.RawMessage `json:"manifest"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(persisted.Body.Bytes(), &persistedData))
	assert.Equal(t, 1, persistedData.Data.Revision)
	assert.Equal(t, manifest, string(persistedData.Data.Manifest))

	firstUpdate := `{"version":1,"canvas":null,"assets":null,"imageLogs":[{"id":"first"}],"videoLogs":[]}`
	firstUpdateResponse := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":1,"manifest":`+firstUpdate+`}`)
	require.Equal(t, http.StatusOK, firstUpdateResponse.Code)
	secondUpdate := `{"version":1,"canvas":null,"assets":null,"imageLogs":[{"id":"second"}],"videoLogs":[]}`
	conflict := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":1,"manifest":`+secondUpdate+`}`)
	assert.Equal(t, http.StatusConflict, conflict.Code)
	assert.Contains(t, conflict.Body.String(), "WORKSPACE_CONFLICT")
	preserved := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/workspace", "")
	require.Equal(t, http.StatusOK, preserved.Code)
	require.NoError(t, common.Unmarshal(preserved.Body.Bytes(), &persistedData))
	assert.Equal(t, 2, persistedData.Data.Revision)
	assert.Equal(t, firstUpdate, string(persistedData.Data.Manifest))

	unsupported := `{"version":1,"canvas":null,"assets":null,"imageLogs":[],"videoLogs":[],"apiKey":"must-not-persist"}`
	unsupportedResponse := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":2,"manifest":`+unsupported+`}`)
	assert.Equal(t, http.StatusBadRequest, unsupportedResponse.Code)

	missingFile := `{"version":1,"canvas":null,"assets":{"assets":[{"data":{"storageKey":"image:missing"}}]},"imageLogs":[],"videoLogs":[]}`
	missingFileResponse := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":2,"manifest":`+missingFile+`}`)
	assert.Equal(t, http.StatusBadRequest, missingFileResponse.Code)
	preservedAfterInvalid := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/workspace", "")
	require.NoError(t, common.Unmarshal(preservedAfterInvalid.Body.Bytes(), &persistedData))
	assert.Equal(t, 2, persistedData.Data.Revision)
	assert.Equal(t, firstUpdate, string(persistedData.Data.Manifest))

	content := []byte("workspace image")
	digest := sha256.Sum256(content)
	hash := hex.EncodeToString(digest[:])
	badHashRequest := httptest.NewRequest(http.MethodPut, "/storage/files/image:bad-hash", bytes.NewReader(content))
	badHashRequest.Header.Set("Authorization", "Bearer "+token)
	badHashRequest.Header.Set("X-Content-SHA256", strings.Repeat("0", sha256.Size*2))
	badHashResponse := httptest.NewRecorder()
	router.ServeHTTP(badHashResponse, badHashRequest)
	assert.Equal(t, http.StatusBadRequest, badHashResponse.Code)
	assert.Contains(t, badHashResponse.Body.String(), "FILE_INVALID")
	noReceipt := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/files/image:bad-hash", "")
	assert.Equal(t, http.StatusNotFound, noReceipt.Code)

	fileRequest := httptest.NewRequest(http.MethodPut, "/storage/files/image:one", bytes.NewReader(content))
	fileRequest.Header.Set("Authorization", "Bearer "+token)
	fileRequest.Header.Set("Content-Type", "image/png")
	fileRequest.Header.Set("X-Content-SHA256", hash)
	fileResponse := httptest.NewRecorder()
	router.ServeHTTP(fileResponse, fileRequest)
	require.Equal(t, http.StatusOK, fileResponse.Code)

	updatedManifest := `{"version":1,"canvas":null,"assets":{"assets":[{"data":{"storageKey":"image:one"}}]},"imageLogs":[],"videoLogs":[]}`
	updated := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":2,"manifest":`+updatedManifest+`}`)
	assert.Equal(t, http.StatusOK, updated.Code)

	idempotentRequest := httptest.NewRequest(http.MethodPut, "/storage/files/image:one", bytes.NewReader(content))
	idempotentRequest.Header.Set("Authorization", "Bearer "+token)
	idempotentRequest.Header.Set("X-Content-SHA256", hash)
	idempotentResponse := httptest.NewRecorder()
	router.ServeHTTP(idempotentResponse, idempotentRequest)
	assert.Equal(t, http.StatusOK, idempotentResponse.Code)

	different := []byte("different content")
	differentHash := sha256.Sum256(different)
	conflictRequest := httptest.NewRequest(http.MethodPut, "/storage/files/image:one", bytes.NewReader(different))
	conflictRequest.Header.Set("Authorization", "Bearer "+token)
	conflictRequest.Header.Set("X-Content-SHA256", hex.EncodeToString(differentHash[:]))
	conflictResponse := httptest.NewRecorder()
	router.ServeHTTP(conflictResponse, conflictRequest)
	assert.Equal(t, http.StatusConflict, conflictResponse.Code)
	assert.Contains(t, conflictResponse.Body.String(), "FILE_CONFLICT")

	read := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/files/image:one", "")
	assert.Equal(t, http.StatusOK, read.Code)
	assert.Equal(t, content, read.Body.Bytes())
	assert.Equal(t, hash, read.Header().Get("X-Content-SHA256"))
}

func TestVideoStudioWorkspaceFilesAreAccountScoped(t *testing.T) {
	router, token, _, _ := setupVideoStudioWorkspaceTest(t)
	primaryManifest := `{"version":1,"canvas":null,"assets":null,"imageLogs":[],"videoLogs":[]}`
	primarySave := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":0,"manifest":`+primaryManifest+`}`)
	require.Equal(t, http.StatusOK, primarySave.Code)
	other := &model.User{Username: "workspace-other", Password: "unused", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "workspace-other-aff", AuthVersion: 1}
	require.NoError(t, model.DB.Create(other).Error)
	otherSession := &model.UserSession{SID: "workspace-other-session", UserID: other.Id, Version: 1, UserAuthVersion: 1, Status: model.UserSessionStatusActive, RefreshHash: "unused", LoginMethod: "password", CreatedAt: time.Now().Unix(), LastActiveAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, model.CreateUserSession(otherSession))
	otherToken, _, err := service.IssueVideoStudioToken(service.AuthIdentity{UserID: other.Id, SessionID: otherSession.SID, UserAuthVersion: 1, SessionVersion: 1}, time.Unix(otherSession.ExpiresAt, 0))
	require.NoError(t, err)
	otherWorkspace := executeVideoStudioWorkspaceRequest(t, router, otherToken, http.MethodGet, "/storage/workspace", "")
	require.Equal(t, http.StatusOK, otherWorkspace.Code)
	assert.Contains(t, otherWorkspace.Body.String(), `"revision":0`)
	assert.Contains(t, otherWorkspace.Body.String(), `"manifest":null`)
	otherManifest := `{"version":1,"canvas":null,"assets":null,"imageLogs":[{"id":"other"}],"videoLogs":[]}`
	otherSave := executeVideoStudioWorkspaceRequest(t, router, otherToken, http.MethodPut, "/storage/workspace", `{"baseRevision":0,"manifest":`+otherManifest+`}`)
	require.Equal(t, http.StatusOK, otherSave.Code)
	primaryWorkspace := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/workspace", "")
	assert.Contains(t, primaryWorkspace.Body.String(), primaryManifest)
	assert.Contains(t, primaryWorkspace.Body.String(), `"revision":1`)

	content := []byte("private")
	digest := sha256.Sum256(content)
	put := httptest.NewRequest(http.MethodPut, "/storage/files/image:private", bytes.NewReader(content))
	put.Header.Set("Authorization", "Bearer "+token)
	put.Header.Set("X-Content-SHA256", hex.EncodeToString(digest[:]))
	putResponse := httptest.NewRecorder()
	router.ServeHTTP(putResponse, put)
	require.Equal(t, http.StatusOK, putResponse.Code)

	otherRead := executeVideoStudioWorkspaceRequest(t, router, otherToken, http.MethodGet, "/storage/files/image:private", "")
	assert.Equal(t, http.StatusNotFound, otherRead.Code)
}

func TestVideoStudioWorkspacePromptSkillsRoundTripAndLegacySave(t *testing.T) {
	router, token, _, _ := setupVideoStudioWorkspaceTest(t)
	skills := `[{"id":"uploaded-skill","name":"Video prompt","instructions":"Use the reference image and describe camera motion."}]`
	manifest := `{"version":1,"canvas":null,"assets":null,"imageLogs":[],"videoLogs":[],"promptSkills":` + skills + `}`
	saved := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":0,"manifest":`+manifest+`}`)
	require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())

	var snapshot struct {
		Data struct {
			Revision int                        `json:"revision"`
			Manifest map[string]json.RawMessage `json:"manifest"`
		} `json:"data"`
	}
	read := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/workspace", "")
	require.Equal(t, http.StatusOK, read.Code)
	require.NoError(t, common.Unmarshal(read.Body.Bytes(), &snapshot))
	assert.JSONEq(t, skills, string(snapshot.Data.Manifest["promptSkills"]))
	assert.Equal(t, 1, snapshot.Data.Revision)

	legacy := `{"version":1,"canvas":null,"assets":null,"imageLogs":[{"id":"legacy-update"}],"videoLogs":[]}`
	updated := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":1,"manifest":`+legacy+`}`)
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())
	require.NoError(t, common.Unmarshal(updated.Body.Bytes(), &snapshot))
	assert.Equal(t, 2, snapshot.Data.Revision)
	assert.JSONEq(t, skills, string(snapshot.Data.Manifest["promptSkills"]))
	assert.JSONEq(t, `[{"id":"legacy-update"}]`, string(snapshot.Data.Manifest["imageLogs"]))

	stale := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":1,"manifest":`+manifest+`}`)
	assert.Equal(t, http.StatusConflict, stale.Code)
	cleared := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", `{"baseRevision":2,"manifest":{"version":1,"canvas":null,"assets":null,"imageLogs":[],"videoLogs":[],"promptSkills":[]}}`)
	require.Equal(t, http.StatusOK, cleared.Code)
	require.NoError(t, common.Unmarshal(cleared.Body.Bytes(), &snapshot))
	assert.Equal(t, 3, snapshot.Data.Revision)
	assert.JSONEq(t, `[]`, string(snapshot.Data.Manifest["promptSkills"]))
}

func TestVideoStudioWorkspaceRejectsInvalidPromptSkills(t *testing.T) {
	router, token, _, _ := setupVideoStudioWorkspaceTest(t)
	for _, skills := range []string{
		`null`, `{}`, `[null]`, `["text"]`,
		`[{"id":"skill","name":"Name"}]`,
		`[{"id":"","name":"Name","instructions":"Prompt"}]`,
		`[{"id":"skill","name":" ","instructions":"Prompt"}]`,
		`[{"id":"skill","name":"Name","instructions":1}]`,
	} {
		t.Run(skills, func(t *testing.T) {
			body := `{"baseRevision":0,"manifest":{"version":1,"canvas":null,"assets":null,"imageLogs":[],"videoLogs":[],"promptSkills":` + skills + `}}`
			response := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodPut, "/storage/workspace", body)
			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.Contains(t, response.Body.String(), "WORKSPACE_INVALID")
		})
	}
	read := executeVideoStudioWorkspaceRequest(t, router, token, http.MethodGet, "/storage/workspace", "")
	assert.Equal(t, http.StatusOK, read.Code)
	assert.Contains(t, read.Body.String(), `"revision":0`)
}
