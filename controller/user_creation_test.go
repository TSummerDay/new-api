package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupUserCreationTest(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, dialect := openTaskDialectDatabase(t, &model.UserCreation{}, &model.Task{})
	oldDB, oldMain, oldLog := model.DB, common.MainDatabaseType(), common.LogDatabaseType()
	model.DB = db
	common.SetDatabaseTypes(dialect, dialect)
	t.Cleanup(func() {
		model.DB = oldDB
		common.SetDatabaseTypes(oldMain, oldLog)
	})
	return db
}

func TestRecordUserCreationRecordsOnlyDeliveredResults(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		query    string
		status   int
		response string
		want     []model.UserCreation
	}{
		{
			name:     "video submission records the public task id",
			kind:     model.UserCreationKindVideo,
			query:    "?source=agent",
			status:   http.StatusOK,
			response: `{"id":"task_public_1","object":"video","status":"queued"}`,
			want:     []model.UserCreation{{Kind: model.UserCreationKindVideo, Source: model.UserCreationSourceAgent, TaskId: "task_public_1"}},
		},
		{
			name:     "image array records url entries only",
			kind:     model.UserCreationKindImage,
			status:   http.StatusOK,
			response: `{"data":[{"url":"https://cdn.example/1.png"},{"b64_json":"aGk="},{"revised_prompt":"x"},{"url":"https://cdn.example/2.png"}]}`,
			want: []model.UserCreation{
				{Kind: model.UserCreationKindImage, Source: model.UserCreationSourceWorkbench, ResultURL: "https://cdn.example/1.png"},
				{Kind: model.UserCreationKindImage, Source: model.UserCreationSourceWorkbench, ResultURL: "https://cdn.example/2.png"},
			},
		},
		{
			name:     "object shaped image data records one image",
			kind:     model.UserCreationKindImage,
			query:    "?source=unknown",
			status:   http.StatusOK,
			response: `{"data":{"url":"https://cdn.example/3.png"}}`,
			want:     []model.UserCreation{{Kind: model.UserCreationKindImage, Source: model.UserCreationSourceWorkbench, ResultURL: "https://cdn.example/3.png"}},
		},
		{
			name:     "failed generation records nothing",
			kind:     model.UserCreationKindVideo,
			status:   http.StatusBadRequest,
			response: `{"id":"task_rejected","error":{"message":"bad request"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupUserCreationTest(t)
			engine := gin.New()
			engine.POST("/pg/generate", func(c *gin.Context) { c.Set("id", 7) }, RecordUserCreation(tt.kind), func(c *gin.Context) {
				c.Data(tt.status, "application/json", []byte(tt.response))
			})

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/pg/generate"+tt.query, bytes.NewBufferString(`{"model":"seedance","prompt":"a cat"}`))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(recorder, request)
			require.Equal(t, tt.status, recorder.Code)
			assert.JSONEq(t, tt.response, recorder.Body.String())

			var got []model.UserCreation
			require.NoError(t, db.Order("id").Find(&got).Error)
			require.Len(t, got, len(tt.want))
			for i, want := range tt.want {
				assert.Equal(t, 7, got[i].UserId)
				assert.Equal(t, "seedance", got[i].ModelName)
				assert.Equal(t, "a cat", got[i].Prompt)
				assert.Equal(t, want.Kind, got[i].Kind)
				assert.Equal(t, want.Source, got[i].Source)
				assert.Equal(t, want.TaskId, got[i].TaskId)
				assert.Equal(t, want.ResultURL, got[i].ResultURL)
			}
		})
	}
}

func TestSelfUserCreationsAreOwnerScopedAndFollowTaskStatus(t *testing.T) {
	db := setupUserCreationTest(t)
	require.NoError(t, db.Create([]*model.Task{
		{TaskID: "task_running", Platform: "doubao", UserId: 7, Status: model.TaskStatusInProgress, Progress: "50%"},
		{TaskID: "task_failed", Platform: "doubao", UserId: 7, Status: model.TaskStatusFailure, Progress: "100%", FailReason: "content rejected"},
	}).Error)
	image := &model.UserCreation{UserId: 7, Kind: model.UserCreationKindImage, Source: model.UserCreationSourceAgent, ModelName: "seedream", ResultURL: "https://cdn.example/1.png"}
	running := &model.UserCreation{UserId: 7, Kind: model.UserCreationKindVideo, Source: model.UserCreationSourceWorkbench, ModelName: "seedance", TaskId: "task_running"}
	failed := &model.UserCreation{UserId: 7, Kind: model.UserCreationKindVideo, Source: model.UserCreationSourceWorkbench, ModelName: "seedance", TaskId: "task_failed"}
	// Another user's creation pointing at user 7's task must not reveal it.
	foreign := &model.UserCreation{UserId: 8, Kind: model.UserCreationKindVideo, Source: model.UserCreationSourceWorkbench, ModelName: "seedance", TaskId: "task_failed"}
	require.NoError(t, model.CreateUserCreations([]*model.UserCreation{image, running, failed, foreign}))

	engine := gin.New()
	authenticate := func(c *gin.Context) {
		userId := 7
		if c.GetHeader("X-Test-User") == "8" {
			userId = 8
		}
		c.Set("id", userId)
	}
	engine.GET("/api/creations/self", authenticate, GetSelfUserCreations)
	engine.GET("/api/creations/self/:id", authenticate, GetSelfUserCreation)
	engine.DELETE("/api/creations/self/:id", authenticate, DeleteSelfUserCreation)
	call := func(method, path, user string) map[string]any {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, path, nil)
		request.Header.Set("X-Test-User", user)
		engine.ServeHTTP(recorder, request)
		var body map[string]any
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &body))
		return body
	}

	list := call(http.MethodGet, "/api/creations/self", "7")
	require.Equal(t, true, list["success"])
	items := list["data"].(map[string]any)["items"].([]any)
	require.Len(t, items, 3)
	statuses := map[float64]map[string]any{}
	for _, raw := range items {
		item := raw.(map[string]any)
		statuses[item["id"].(float64)] = item
	}
	assert.Equal(t, "succeeded", statuses[float64(image.Id)]["status"])
	assert.Equal(t, "pending", statuses[float64(running.Id)]["status"])
	assert.Equal(t, "50%", statuses[float64(running.Id)]["progress"])
	assert.Equal(t, "failed", statuses[float64(failed.Id)]["status"])
	assert.Equal(t, "content rejected", statuses[float64(failed.Id)]["fail_reason"])

	videos := call(http.MethodGet, "/api/creations/self?kind=video", "7")
	assert.Len(t, videos["data"].(map[string]any)["items"].([]any), 2)
	assert.Equal(t, false, call(http.MethodGet, "/api/creations/self?kind=audio", "7")["success"])

	foreignView := call(http.MethodGet, "/api/creations/self/"+strconv.Itoa(foreign.Id), "8")
	require.Equal(t, true, foreignView["success"])
	assert.Equal(t, "unknown", foreignView["data"].(map[string]any)["status"])
	assert.Nil(t, foreignView["data"].(map[string]any)["fail_reason"])

	assert.Equal(t, false, call(http.MethodGet, "/api/creations/self/"+strconv.Itoa(foreign.Id), "7")["success"])
	assert.Equal(t, false, call(http.MethodDelete, "/api/creations/self/"+strconv.Itoa(foreign.Id), "7")["success"])
	var remaining int64
	require.NoError(t, db.Model(&model.UserCreation{}).Where("id = ?", foreign.Id).Count(&remaining).Error)
	assert.Equal(t, int64(1), remaining)

	assert.Equal(t, true, call(http.MethodDelete, "/api/creations/self/"+strconv.Itoa(image.Id), "7")["success"])
	require.NoError(t, db.Model(&model.UserCreation{}).Where("id = ?", image.Id).Count(&remaining).Error)
	assert.Equal(t, int64(0), remaining)
}
