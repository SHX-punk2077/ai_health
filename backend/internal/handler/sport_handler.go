package handler

import (
	"github.com/gin-gonic/gin"

	"ai_health/internal/pkg/errcode"
	"ai_health/internal/pkg/response"
	"ai_health/internal/service"
)

// SportHandler 运动类型、运动建议、疾病禁忌。
//
// TODO 全部方法待接入 SportService，当前统一返回 10003。
type SportHandler struct {
	svc *service.SportService
}

func NewSportHandler(svc *service.SportService) *SportHandler {
	return &SportHandler{svc: svc}
}

// ListTypes 运动类型字典
func (h *SportHandler) ListTypes(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// ListInfos 运动建议列表
func (h *SportHandler) ListInfos(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// ListDiseases 疾病运动禁忌
func (h *SportHandler) ListDiseases(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// CreateInfo 新增运动建议
func (h *SportHandler) CreateInfo(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}

// UpdateInfo 更新运动建议
func (h *SportHandler) UpdateInfo(c *gin.Context) {
	response.Fail(c, errcode.ErrNotImpl)
}
