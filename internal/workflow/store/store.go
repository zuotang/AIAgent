package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"agent-langchain/internal/workflow/dsl"
	"agent-langchain/internal/workflow/engine"

	_ "modernc.org/sqlite"
)

// WorkflowStore 工作流存储
type WorkflowStore struct {
	db *gorm.DB
}

// WorkflowRecord 工作流记录
type WorkflowRecord struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	UserID       string    `gorm:"index" json:"user_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	WorkflowJSON string    `gorm:"type:text" json:"workflow_json"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (WorkflowRecord) TableName() string {
	return "workflows"
}

// WorkflowExecutionRecord 工作流执行记录
type WorkflowExecutionRecord struct {
	ID          string    `gorm:"primaryKey" json:"id"`
	WorkflowID  string    `gorm:"index" json:"workflow_id"`
	UserID      string    `gorm:"index" json:"user_id"`
	Status      string    `json:"status"`
	TraceJSON   string    `gorm:"type:text" json:"trace_json"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TableName 指定表名
func (WorkflowExecutionRecord) TableName() string {
	return "workflow_executions"
}

// NewWorkflowStore 创建工作流存储
func NewWorkflowStore(dbPath string) (*WorkflowStore, error) {
	// 使用 database/sql 打开连接
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 配置 GORM
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	// 创建 GORM 实例
	db, err := gorm.Open(sqlite.Dialector{
		Conn: sqlDB,
	}, config)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize GORM: %w", err)
	}

	store := &WorkflowStore{db: db}

	// 初始化表
	if err := store.init(); err != nil {
		return nil, err
	}

	return store, nil
}

// init 初始化数据库表
func (s *WorkflowStore) init() error {
	// 自动迁移表结构
	if err := s.db.AutoMigrate(&WorkflowRecord{}, &WorkflowExecutionRecord{}); err != nil {
		return fmt.Errorf("failed to migrate tables: %w", err)
	}
	return nil
}

// Close 关闭数据库连接
func (s *WorkflowStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// SaveWorkflow 保存工作流
func (s *WorkflowStore) SaveWorkflow(userID string, workflow *dsl.Workflow, name, description string) error {
	workflowJSON, err := json.Marshal(workflow)
	if err != nil {
		return fmt.Errorf("failed to marshal workflow: %w", err)
	}

	record := WorkflowRecord{
		ID:           workflow.Meta.ID,
		UserID:       userID,
		Name:         name,
		Description:  description,
		WorkflowJSON: string(workflowJSON),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// 使用 GORM 的 Save 方法（如果存在则更新，不存在则创建）
	result := s.db.Save(&record)
	if result.Error != nil {
		return fmt.Errorf("failed to save workflow: %w", result.Error)
	}

	return nil
}

// GetWorkflow 获取工作流
func (s *WorkflowStore) GetWorkflow(id string) (*dsl.Workflow, *WorkflowRecord, error) {
	var record WorkflowRecord
	result := s.db.Where("id = ?", id).First(&record)
	if result.Error != nil {
		return nil, nil, result.Error
	}

	var workflow dsl.Workflow
	if err := json.Unmarshal([]byte(record.WorkflowJSON), &workflow); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal workflow: %w", err)
	}

	return &workflow, &record, nil
}

// ListWorkflows 列出工作流
func (s *WorkflowStore) ListWorkflows(userID string, page, limit int) ([]WorkflowRecord, int64, error) {
	var records []WorkflowRecord
	var total int64

	query := s.db.Model(&WorkflowRecord{})
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * limit
	result := query.Order("updated_at DESC").Offset(offset).Limit(limit).Find(&records)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	return records, total, nil
}

// DeleteWorkflow 删除工作流
func (s *WorkflowStore) DeleteWorkflow(id string) error {
	result := s.db.Where("id = ?", id).Delete(&WorkflowRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SaveExecution 保存执行记录
func (s *WorkflowStore) SaveExecution(workflowID, userID string, trace *engine.RunTrace) error {
	traceJSON, err := json.Marshal(trace)
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	completedAt := time.Now()
	record := WorkflowExecutionRecord{
		ID:          trace.WorkflowID + "-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		WorkflowID:  workflowID,
		UserID:      userID,
		Status:      string(trace.Status),
		TraceJSON:   string(traceJSON),
		StartedAt:   trace.StartTime,
		CompletedAt: &completedAt,
	}

	result := s.db.Create(&record)
	if result.Error != nil {
		return fmt.Errorf("failed to save execution: %w", result.Error)
	}

	return nil
}

// GetExecution 获取执行记录
func (s *WorkflowStore) GetExecution(id string) (*engine.RunTrace, error) {
	var record WorkflowExecutionRecord
	result := s.db.Where("id = ?", id).First(&record)
	if result.Error != nil {
		return nil, result.Error
	}

	var trace engine.RunTrace
	if err := json.Unmarshal([]byte(record.TraceJSON), &trace); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trace: %w", err)
	}

	return &trace, nil
}

// ListExecutions 列出执行记录
func (s *WorkflowStore) ListExecutions(workflowID string, page, limit int) ([]WorkflowExecutionRecord, int64, error) {
	var records []WorkflowExecutionRecord
	var total int64

	query := s.db.Model(&WorkflowExecutionRecord{})
	if workflowID != "" {
		query = query.Where("workflow_id = ?", workflowID)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * limit
	result := query.Order("started_at DESC").Offset(offset).Limit(limit).Find(&records)
	if result.Error != nil {
		return nil, 0, result.Error
	}

	return records, total, nil
}

