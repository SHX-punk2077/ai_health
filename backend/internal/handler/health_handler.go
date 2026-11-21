package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// HealthHandler 健康档案与健康记录。
//
// TODO 全部方法待接入 HealthService，当前统一返回 10003。
type HealthHandler struct {
	svc *service.HealthService
}

func NewHealthHandler(svc *service.HealthService) *HealthHandler {
	return &HealthHandler{svc: svc}
}

// GetProfile 获取我的健康档案
func (h *HealthHandler) GetProfile(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// SaveProfile 保存/更新健康档案
func (h *HealthHandler) SaveProfile(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// ListRecords 分页查询健康记录
func (h *HealthHandler) ListRecords(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// CreateRecord 新增一条健康记录
func (h *HealthHandler) CreateRecord(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// DeleteRecord 删除健康记录
func (h *HealthHandler) DeleteRecord(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// Trend 指标趋势曲线
func (h *HealthHandler) Trend(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}
