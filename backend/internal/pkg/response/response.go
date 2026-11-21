package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"ai_health/internal/pkg/errcode"
)

// Body 全局统一响应体。
type Body struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

// PageData 分页结构。
type PageData struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

const ctxRequestID = "requestID"

// OK 成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: data, RequestID: c.GetString(ctxRequestID)})
}

// Page 分页响应。
func Page(c *gin.Context, list any, total int64, page, pageSize int) {
	OK(c, PageData{List: list, Total: total, Page: page, PageSize: pageSize})
}

// Fail 失败响应：errcode.Error 映射业务码，其余一律 500。
func Fail(c *gin.Context, err error) {
	var e *errcode.Error
	if errors.As(err, &e) {
		status := http.StatusOK
		switch e.Code / 10000 {
		case 2:
			status = http.StatusUnauthorized
		case 3:
			status = http.StatusBadRequest
		}
		c.JSON(status, Body{Code: e.Code, Message: e.Msg, RequestID: c.GetString(ctxRequestID)})
		return
	}
	c.JSON(http.StatusInternalServerError, Body{
		Code:      errcode.ErrInternal.Code,
		Message:   errcode.ErrInternal.Msg,
		RequestID: c.GetString(ctxRequestID),
	})
}

// Abort 中断请求（中间件使用），只写响应不再执行后续 handler。
func Abort(c *gin.Context, e *errcode.Error) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, Body{Code: e.Code, Message: e.Msg, RequestID: c.GetString(ctxRequestID)})
}
