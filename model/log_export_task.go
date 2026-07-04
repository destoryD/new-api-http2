package model

import (
	"time"

	"gorm.io/gorm"
)

type LogExportTaskStatus string

const (
	LogExportTaskStatusPending LogExportTaskStatus = "pending"
	LogExportTaskStatusRunning LogExportTaskStatus = "running"
	LogExportTaskStatusSuccess LogExportTaskStatus = "success"
	LogExportTaskStatusFailed  LogExportTaskStatus = "failed"
)

type LogExportTask struct {
	Id          int                 `json:"id" gorm:"primaryKey"`
	CreatedAt   int64               `json:"created_at" gorm:"index"`
	UpdatedAt   int64               `json:"updated_at"`
	FinishedAt  int64               `json:"finished_at" gorm:"index"`
	UserId      int                 `json:"user_id" gorm:"index"`
	Username    string              `json:"username" gorm:"index;default:''"`
	IsAdmin     bool                `json:"is_admin" gorm:"index"`
	Kind        string              `json:"kind" gorm:"type:varchar(32);index"`
	Format      string              `json:"format" gorm:"type:varchar(8)"`
	Status      LogExportTaskStatus `json:"status" gorm:"type:varchar(20);index"`
	Progress    int                 `json:"progress" gorm:"default:0"`
	Rows        int                 `json:"rows" gorm:"default:0"`
	Filename    string              `json:"filename"`
	FilePath    string              `json:"-" gorm:"type:text"`
	FileSize    int64               `json:"file_size" gorm:"default:0"`
	Error       string              `json:"error" gorm:"type:text"`
	QueryParams string              `json:"query_params" gorm:"type:text"`
}

func (t *LogExportTask) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Unix()
	if t.CreatedAt == 0 {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	return nil
}

func (t *LogExportTask) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = time.Now().Unix()
	return nil
}

func CreateLogExportTask(task *LogExportTask) error {
	return DB.Create(task).Error
}

func GetLogExportTaskByID(id int) (*LogExportTask, error) {
	var task LogExportTask
	err := DB.Where("id = ?", id).First(&task).Error
	return &task, err
}

func GetLogExportTasks(userId int, isAdmin bool, limit int) ([]*LogExportTask, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var tasks []*LogExportTask
	query := DB.Model(&LogExportTask{}).Order("id desc").Limit(limit)
	if !isAdmin {
		query = query.Where("user_id = ? AND is_admin = ?", userId, false)
	}
	err := query.Find(&tasks).Error
	return tasks, err
}

func UpdateLogExportTask(taskId int, values map[string]interface{}) error {
	return DB.Model(&LogExportTask{}).Where("id = ?", taskId).Updates(values).Error
}

func GetExpiredLogExportTasks(cutoff int64, limit int) ([]*LogExportTask, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var tasks []*LogExportTask
	err := DB.Where("created_at < ?", cutoff).Order("id asc").Limit(limit).Find(&tasks).Error
	return tasks, err
}

func DeleteLogExportTaskByID(taskId int) error {
	return DB.Delete(&LogExportTask{}, taskId).Error
}
