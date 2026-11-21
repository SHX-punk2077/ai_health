package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"ai_health/internal/dto/request"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// HealthHandler 健康档案、健康记录与趋势。
// 所有接口都只使用 JWT 中的 uid，请求体不包含也无法覆盖数据归属。
type HealthHandler struct {
	svc *service.HealthService
}

func NewHealthHandler(svc *service.HealthService) *HealthHandler {
	return &HealthHandler{svc: svc}
}

// GetProfile 获取我的健康档案
// @Summary      获取当前用户健康档案
// @Tags         health
// @Security     Bearer
// @Success      200 {object} response.Body{data=response.Profile}
// @Failure      40002 "尚未创建健康档案"
// @Router       /health/profile [get]
func (h *HealthHandler) GetProfile(c *gin.Context) {
	profile, err := h.svc.GetProfile(c.Request.Context(), currentUID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, profile)
}

// CreateProfile 创建当前用户健康档案（严格创建，一人一份）
// @Summary      创建健康档案
// @Tags         health
// @Security     Bearer
// @Accept       json
// @Produce      json
// @Param        body body request.ProfileCreateReq true "档案字段"
// @Success      200 {object} response.Body{data=response.Profile}
// @Failure      40003 "档案已存在"
// @Router       /health/profile [post]
func (h *HealthHandler) CreateProfile(c *gin.Context) {
	var req request.ProfileCreateReq
	if !bindJSON(c, &req) {
		return
	}
	profile, err := h.svc.CreateProfile(c.Request.Context(), currentUID(c), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, profile)
}

// UpdateProfile 修改当前用户健康档案（部分更新）
// @Summary      修改健康档案
// @Tags         health
// @Security     Bearer
// @Accept       json
// @Produce      json
// @Param        body body request.ProfileUpdateReq true "需要修改的字段"
// @Success      200 {object} response.Body{data=response.Profile}
// @Failure      40002 "尚未创建健康档案"
// @Router       /health/profile [put]
func (h *HealthHandler) UpdateProfile(c *gin.Context) {
	var req request.ProfileUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	profile, err := h.svc.UpdateProfile(c.Request.Context(), currentUID(c), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, profile)
}

// SaveProfile 兼容旧方法名，语义与 UpdateProfile 相同。
func (h *HealthHandler) SaveProfile(c *gin.Context) { h.UpdateProfile(c) }

// ListRecords 分页查询健康记录
// @Summary      查询当前用户健康记录
// @Tags         health
// @Security     Bearer
// @Param        page query int false "页码，默认 1"
// @Param        pageSize query int false "每页条数，默认 10，最大 100"
// @Param        startAt query string false "开始日期 YYYY-MM-DD（含）"
// @Param        endAt query string false "结束日期 YYYY-MM-DD（含）"
// @Param        metric query string false "指标名，仅返回该指标非空的记录"
// @Success      200 {object} response.Body{data=response.PageData{list=[]response.Record}}
// @Router       /health/records [get]
func (h *HealthHandler) ListRecords(c *gin.Context) {
	var q request.RecordQuery
	if !bindQuery(c, &q) {
		return
	}
	list, total, page, pageSize, err := h.svc.ListRecords(c.Request.Context(), currentUID(c), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Page(c, list, total, page, pageSize)
}

// CreateRecord 新增一条健康记录
// @Summary      新增健康记录
// @Tags         health
// @Security     Bearer
// @Accept       json
// @Produce      json
// @Param        body body request.RecordCreateReq true "测量数据"
// @Success      200 {object} response.Body{data=response.Record}
// @Router       /health/records [post]
func (h *HealthHandler) CreateRecord(c *gin.Context) {
	var req request.RecordCreateReq
	if !bindJSON(c, &req) {
		return
	}
	record, err := h.svc.CreateRecord(c.Request.Context(), currentUID(c), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, record)
}

// GetRecord 查询一条健康记录
// @Summary      查询健康记录详情
// @Tags         health
// @Security     Bearer
// @Param        id path int true "记录 ID"
// @Success      200 {object} response.Body{data=response.Record}
// @Failure      40004 "记录不存在或不属于当前用户"
// @Router       /health/records/{id} [get]
func (h *HealthHandler) GetRecord(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	record, err := h.svc.GetRecord(c.Request.Context(), currentUID(c), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, record)
}

// UpdateRecord 部分更新一条健康记录
// @Summary      修改健康记录
// @Tags         health
// @Security     Bearer
// @Accept       json
// @Produce      json
// @Param        id path int true "记录 ID"
// @Param        body body request.RecordUpdateReq true "需要修改的字段"
// @Success      200 {object} response.Body{data=response.Record}
// @Failure      40004 "记录不存在或不属于当前用户"
// @Router       /health/records/{id} [put]
func (h *HealthHandler) UpdateRecord(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req request.RecordUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	record, err := h.svc.UpdateRecord(c.Request.Context(), currentUID(c), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, record)
}

// DeleteRecord 删除一条健康记录（物理删除）
// @Summary      删除健康记录
// @Tags         health
// @Security     Bearer
// @Param        id path int true "记录 ID"
// @Success      200 {object} response.Body
// @Failure      40004 "记录不存在或不属于当前用户"
// @Router       /health/records/{id} [delete]
func (h *HealthHandler) DeleteRecord(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteRecord(c.Request.Context(), currentUID(c), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"id": strconv.FormatUint(id, 10)})
}

// Summary 首页健康数据概览
// @Summary      获取当前用户健康数据概览
// @Tags         health
// @Security     Bearer
// @Success      200 {object} response.Body{data=response.HealthSummary}
// @Router       /health/summary [get]
func (h *HealthHandler) Summary(c *gin.Context) {
	summary, err := h.svc.Summary(c.Request.Context(), currentUID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, summary)
}

// Trend 指标趋势曲线
// @Summary      查询健康指标趋势
// @Tags         health
// @Security     Bearer
// @Param        metric query string true "指标名，见文档"
// @Param        startDate query string false "开始日期 YYYY-MM-DD，默认最近 90 天"
// @Param        endDate query string false "结束日期 YYYY-MM-DD，默认今天"
// @Param        granularity query string false "day|week|month，默认 day"
// @Param        limit query int false "只返回最近 N 个点，最大 1000"
// @Success      200 {object} response.Body{data=response.Trend}
// @Router       /health/trends [get]
func (h *HealthHandler) Trend(c *gin.Context) {
	var q request.TrendQuery
	if !bindQuery(c, &q) {
		return
	}
	trend, err := h.svc.Trend(c.Request.Context(), currentUID(c), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, trend)
}
