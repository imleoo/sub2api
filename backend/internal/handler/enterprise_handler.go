package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// EnterpriseHandler 处理企业客户自助升级相关请求。
// zhiguofan fork-only: 企业组织与额度分配（Team 协作 v2）。
type EnterpriseHandler struct {
	enterpriseService *service.EnterpriseService
}

// NewEnterpriseHandler 创建 EnterpriseHandler。
func NewEnterpriseHandler(enterpriseService *service.EnterpriseService) *EnterpriseHandler {
	return &EnterpriseHandler{enterpriseService: enterpriseService}
}

type enterpriseProfileDTO struct {
	CompanyName  string `json:"company_name"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Industry     string `json:"industry"`
	CreatedAt    string `json:"created_at"`
}

type upgradeEnterpriseRequest struct {
	CompanyName  string `json:"company_name" binding:"required"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Industry     string `json:"industry"`
}

// Upgrade 补充企业信息，自助即时升级为企业客户。
// POST /user/enterprise/profile
func (h *EnterpriseHandler) Upgrade(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req upgradeEnterpriseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	profile, err := h.enterpriseService.Upgrade(c.Request.Context(), subject.UserID, service.EnterpriseUpgradeInput{
		CompanyName:  req.CompanyName,
		ContactName:  req.ContactName,
		ContactPhone: req.ContactPhone,
		Industry:     req.Industry,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, enterpriseProfileDTO{
		CompanyName:  profile.CompanyName,
		ContactName:  profile.ContactName,
		ContactPhone: profile.ContactPhone,
		Industry:     profile.Industry,
		CreatedAt:    profile.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetProfile 查询当前用户的企业资料；非企业客户返回 data:null。
// GET /user/enterprise/profile
func (h *EnterpriseHandler) GetProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	profile, err := h.enterpriseService.GetProfile(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if profile == nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, enterpriseProfileDTO{
		CompanyName:  profile.CompanyName,
		ContactName:  profile.ContactName,
		ContactPhone: profile.ContactPhone,
		Industry:     profile.Industry,
		CreatedAt:    profile.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}
