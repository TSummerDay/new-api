package controller

import (
	"bytes"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// maxCreationResponseBytes bounds the copy of a generation response kept to
// record the creation; larger responses (inline base64 images) are not
// recorded.
const maxCreationResponseBytes = 8 << 20

type creationResponseWriter struct {
	gin.ResponseWriter
	body     bytes.Buffer
	overflow bool
}

func (w *creationResponseWriter) Write(b []byte) (int, error) {
	if !w.overflow {
		if w.body.Len()+len(b) > maxCreationResponseBytes {
			w.overflow = true
			w.body.Reset()
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *creationResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// RecordUserCreation records what a console generation request produced.
// Rows come only from the relay's own successful response, never from
// client-supplied URLs.
func RecordUserCreation(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		source := model.UserCreationSourceWorkbench
		if c.Query("source") == model.UserCreationSourceAgent {
			source = model.UserCreationSourceAgent
		}
		var modelName, prompt string
		if storage, err := common.GetBodyStorage(c); err == nil {
			if body, err := storage.Bytes(); err == nil {
				values := gjson.GetManyBytes(body, "model", "prompt")
				modelName, prompt = values[0].String(), values[1].String()
			}
		}
		writer := &creationResponseWriter{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()
		if writer.Status() != http.StatusOK || writer.overflow {
			return
		}

		base := model.UserCreation{UserId: c.GetInt("id"), Kind: kind, Source: source, ModelName: modelName, Prompt: prompt}
		var creations []*model.UserCreation
		response := gjson.ParseBytes(writer.body.Bytes())
		switch kind {
		case model.UserCreationKindImage:
			// Array also yields a single-object data, which some providers return.
			for _, entry := range response.Get("data").Array() {
				if url := entry.Get("url").String(); url != "" {
					creation := base
					creation.ResultURL = url
					creations = append(creations, &creation)
				}
			}
		case model.UserCreationKindVideo:
			taskId := response.Get("id").String()
			if taskId == "" {
				taskId = response.Get("task_id").String()
			}
			if taskId != "" {
				creation := base
				creation.TaskId = taskId
				creations = append(creations, &creation)
			}
		}
		if err := model.CreateUserCreations(creations); err != nil {
			logger.LogError(c, common.LogText("record user creation failed: %s", err.Error()))
		}
	}
}

type userCreationItem struct {
	*model.UserCreation
	Status     string `json:"status"`
	Progress   string `json:"progress,omitempty"`
	FailReason string `json:"fail_reason,omitempty"`
}

// userCreationItems attaches the generation status: images are complete when
// recorded, videos follow their task.
func userCreationItems(userId int, creations []*model.UserCreation) ([]userCreationItem, error) {
	var taskIds []string
	for _, creation := range creations {
		if creation.TaskId != "" {
			taskIds = append(taskIds, creation.TaskId)
		}
	}
	tasks, err := model.GetUserTasksByTaskIds(userId, taskIds)
	if err != nil {
		return nil, err
	}
	tasksById := make(map[string]*model.Task, len(tasks))
	for _, task := range tasks {
		tasksById[task.TaskID] = task
	}
	items := make([]userCreationItem, 0, len(creations))
	for _, creation := range creations {
		item := userCreationItem{UserCreation: creation, Status: "succeeded"}
		if creation.TaskId != "" {
			task := tasksById[creation.TaskId]
			switch {
			case task == nil:
				item.Status = "unknown"
			case task.Status == model.TaskStatusSuccess:
				item.Progress = task.Progress
			case task.Status == model.TaskStatusFailure:
				item.Status = "failed"
				item.Progress = task.Progress
				item.FailReason = task.FailReason
			default:
				item.Status = "pending"
				item.Progress = task.Progress
			}
		}
		items = append(items, item)
	}
	return items, nil
}

func GetSelfUserCreations(c *gin.Context) {
	kind := c.Query("kind")
	if kind != "" && kind != model.UserCreationKindImage && kind != model.UserCreationKindVideo {
		common.ApiErrorT(c, "Invalid creation type")
		return
	}
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	creations, total, err := model.ListUserCreations(userId, kind, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items, err := userCreationItems(userId, creations)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetSelfUserCreation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorT(c, "Invalid ID")
		return
	}
	userId := c.GetInt("id")
	creation, found, err := model.GetUserCreation(userId, id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !found {
		common.ApiErrorT(c, "Creation not found")
		return
	}
	items, err := userCreationItems(userId, []*model.UserCreation{creation})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items[0])
}

func DeleteSelfUserCreation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorT(c, "Invalid ID")
		return
	}
	deleted, err := model.DeleteUserCreation(c.GetInt("id"), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !deleted {
		common.ApiErrorT(c, "Creation not found")
		return
	}
	common.ApiSuccess(c, nil)
}
