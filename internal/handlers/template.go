package handlers

import (
	"context"
	"strconv"

	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type TemplateHandler struct {
	templateService TemplateServiceInterface
}

func NewTemplateHandler(templateService TemplateServiceInterface) *TemplateHandler {
	return &TemplateHandler{
		templateService: templateService,
	}
}

func (h *TemplateHandler) Search(c *drift.Context) {
	query := c.QueryParam("q")
	limitStr := c.QueryParam("limit")

	limit := 10
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	ctx := context.Background()
	templates, err := h.templateService.Search(ctx, query, limit)
	if err != nil {
		c.InternalServerError("failed to search templates")
		return
	}

	results := make([]dto.TemplateSearchResult, len(templates))
	for i, t := range templates {
		results[i] = dto.TemplateSearchResult{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Category:    t.Category,
		}
	}

	_ = c.JSON(200, results)
}

func (h *TemplateHandler) Get(c *drift.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		c.BadRequest("invalid template id")
		return
	}

	ctx := context.Background()
	template, err := h.templateService.GetByID(ctx, templateID)
	if err != nil {
		c.NotFound("template not found")
		return
	}

	_ = c.JSON(200, dto.TemplateDetail{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		Category:    template.Category,
		Data:        template.Data,
	})
}

func (h *TemplateHandler) Create(c *drift.Context) {
	var req dto.CreateTemplateRequest
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if req.Name == "" {
		c.BadRequest("name is required")
		return
	}

	if req.Data == nil {
		c.BadRequest("data is required")
		return
	}

	ctx := context.Background()
	template, err := h.templateService.Create(ctx, req.Name, req.Description, req.Category, req.Data)
	if err != nil {
		c.InternalServerError("failed to create template")
		return
	}

	_ = c.JSON(201, dto.TemplateDetail{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		Category:    template.Category,
		Data:        template.Data,
	})
}

func (h *TemplateHandler) Delete(c *drift.Context) {
	templateID, err := uuid.Parse(c.Param("templateId"))
	if err != nil {
		c.BadRequest("invalid template id")
		return
	}

	ctx := context.Background()
	if err := h.templateService.Delete(ctx, templateID); err != nil {
		c.InternalServerError("failed to delete template")
		return
	}

	_ = c.JSON(200, map[string]string{"message": "template deleted"})
}
