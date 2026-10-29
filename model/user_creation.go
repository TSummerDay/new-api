package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	UserCreationKindImage = "image"
	UserCreationKindVideo = "video"

	UserCreationSourceAgent     = "agent"
	UserCreationSourceWorkbench = "workbench"
)

// maxUserCreationPromptRunes keeps prompts within the MySQL TEXT limit.
const maxUserCreationPromptRunes = 4000

// UserCreation is an image or video a user generated from the web console.
// Images keep the delivered URL; videos keep the public task id and read
// their status and artifacts from the task.
type UserCreation struct {
	Id        int    `json:"id"`
	UserId    int    `json:"user_id" gorm:"not null;index:idx_user_creation_list,priority:1"`
	Kind      string `json:"kind" gorm:"type:varchar(16);not null"`
	Source    string `json:"source" gorm:"type:varchar(16);not null"`
	ModelName string `json:"model_name" gorm:"type:varchar(128);not null"`
	Prompt    string `json:"prompt" gorm:"type:text"`
	TaskId    string `json:"task_id" gorm:"type:varchar(191);index"`
	ResultURL string `json:"result_url" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"bigint;index:idx_user_creation_list,priority:2"`
}

func CreateUserCreations(creations []*UserCreation) error {
	if len(creations) == 0 {
		return nil
	}
	now := common.GetTimestamp()
	for _, creation := range creations {
		if prompt := []rune(creation.Prompt); len(prompt) > maxUserCreationPromptRunes {
			creation.Prompt = string(prompt[:maxUserCreationPromptRunes])
		}
		creation.CreatedAt = now
	}
	return DB.Create(creations).Error
}

func ListUserCreations(userId int, kind string, startIdx int, num int) ([]*UserCreation, int64, error) {
	query := DB.Model(&UserCreation{}).Where("user_id = ?", userId)
	if kind != "" {
		query = query.Where("kind = ?", kind)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var creations []*UserCreation
	err := query.Order("created_at desc").Order("id desc").Limit(num).Offset(startIdx).Find(&creations).Error
	return creations, total, err
}

func GetUserCreation(userId int, id int) (*UserCreation, bool, error) {
	var creation UserCreation
	err := DB.Where("id = ? AND user_id = ?", id, userId).First(&creation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &creation, true, nil
}

func DeleteUserCreation(userId int, id int) (bool, error) {
	result := DB.Where("id = ? AND user_id = ?", id, userId).Delete(&UserCreation{})
	return result.RowsAffected > 0, result.Error
}

// GetUserTasksByTaskIds loads the caller's own tasks; ids owned by another
// user are silently absent.
func GetUserTasksByTaskIds(userId int, taskIds []string) ([]*Task, error) {
	if len(taskIds) == 0 {
		return nil, nil
	}
	var tasks []*Task
	err := DB.Where("user_id = ? AND task_id IN ?", userId, taskIds).Find(&tasks).Error
	return tasks, err
}
