package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dimitrije/nikode-api/internal/middleware"
	"github.com/dimitrije/nikode-api/internal/services"
	"github.com/dimitrije/nikode-api/pkg/dto"
	"github.com/google/uuid"
	"github.com/m1z23r/drift/pkg/drift"
)

type AutomationHandler struct {
	collectionService CollectionServiceInterface
	openAPIService    OpenAPIServiceInterface
}

func NewAutomationHandler(collectionService CollectionServiceInterface, openAPIService OpenAPIServiceInterface) *AutomationHandler {
	return &AutomationHandler{
		collectionService: collectionService,
		openAPIService:    openAPIService,
	}
}

func (h *AutomationHandler) UpsertCollection(c *drift.Context) {
	workspaceID := middleware.GetAPIKeyWorkspaceID(c)
	if workspaceID == uuid.Nil {
		c.Unauthorized("not authenticated")
		return
	}

	var req dto.UpsertCollectionRequest
	var specBytes []byte

	contentType := c.GetHeader("Content-Type")
	isYAML := strings.Contains(contentType, "application/yaml") ||
		strings.Contains(contentType, "text/yaml") ||
		strings.Contains(contentType, "application/x-yaml")

	if isYAML {
		// Raw YAML body — the body IS the OpenAPI spec
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.BadRequest("failed to read request body")
			return
		}
		specBytes = body

		// Metadata comes from query params
		req.Name = c.QueryParam("name")
		req.CollectionID = c.QueryParam("collection_id")
		req.Resolution = c.QueryParam("resolution")
	} else {
		// JSON body
		if err := c.BindJSON(&req); err != nil {
			c.BadRequest("invalid request body")
			return
		}

		if len(req.Spec) == 0 {
			c.BadRequest("spec is required")
			return
		}

		// Check if spec is a string (YAML) or JSON object
		var yamlStr string
		if err := json.Unmarshal(req.Spec, &yamlStr); err == nil {
			specBytes = []byte(yamlStr)
		} else {
			specBytes = req.Spec
		}
	}

	// Default resolution to "force"
	if req.Resolution == "" {
		req.Resolution = "force"
	}

	if req.Resolution != "force" && req.Resolution != "clone" && req.Resolution != "fail" {
		c.BadRequest("resolution must be one of: force, clone, fail")
		return
	}

	if len(specBytes) == 0 {
		c.BadRequest("spec is required")
		return
	}

	// Parse the OpenAPI spec (auto-detects JSON/YAML)
	spec, err := h.openAPIService.ParseOpenAPI(specBytes)
	if err != nil {
		c.BadRequest("invalid openapi spec: " + err.Error())
		return
	}

	// Convert to Nikode format
	data, err := h.openAPIService.ConvertToNikode(spec)
	if err != nil {
		c.InternalServerError("failed to convert openapi spec")
		return
	}

	ctx := context.Background()

	// Resolve existing collection: by ID first, then by name
	var existing *struct {
		ID      uuid.UUID
		Name    string
		Version int
	}

	if req.CollectionID != "" {
		collectionID, err := uuid.Parse(req.CollectionID)
		if err != nil {
			c.BadRequest("invalid collection_id")
			return
		}

		col, err := h.collectionService.GetByID(ctx, collectionID)
		if err != nil {
			if errors.Is(err, services.ErrCollectionNotFound) {
				c.NotFound("collection not found")
				return
			}
			c.InternalServerError("failed to look up collection")
			return
		}

		// Verify the collection belongs to this workspace
		if col.WorkspaceID != workspaceID {
			c.NotFound("collection not found")
			return
		}

		existing = &struct {
			ID      uuid.UUID
			Name    string
			Version int
		}{col.ID, col.Name, col.Version}
	} else {
		// Require name when no collection_id
		if strings.TrimSpace(req.Name) == "" {
			c.BadRequest("name or collection_id is required")
			return
		}

		col, err := h.collectionService.GetByWorkspaceAndName(ctx, workspaceID, req.Name)
		if err != nil && !errors.Is(err, services.ErrCollectionNotFound) {
			c.InternalServerError("failed to check existing collection")
			return
		}
		if col != nil {
			existing = &struct {
				ID      uuid.UUID
				Name    string
				Version int
			}{col.ID, col.Name, col.Version}
		}
	}

	// Use existing name as fallback if name not provided
	name := strings.TrimSpace(req.Name)
	if name == "" && existing != nil {
		name = existing.Name
	}

	var response dto.UpsertCollectionResponse

	if existing != nil {
		switch req.Resolution {
		case "fail":
			_ = c.JSON(409, map[string]string{
				"error":   "collection exists",
				"message": fmt.Sprintf("collection %q already exists, use resolution=force or resolution=clone", existing.Name),
			})
			return

		case "clone":
			// Create a new collection with a suffixed name
			cloneName := name + " (copy)"
			newCollection, err := h.collectionService.Create(ctx, workspaceID, cloneName, data, uuid.Nil)
			if err != nil {
				c.InternalServerError("failed to clone collection")
				return
			}
			response = dto.UpsertCollectionResponse{
				ID:          newCollection.ID,
				WorkspaceID: newCollection.WorkspaceID,
				Name:        newCollection.Name,
				Version:     newCollection.Version,
				Created:     true,
			}
			_ = c.JSON(201, response)
			return

		case "force":
			updated, err := h.collectionService.ForceUpdate(ctx, existing.ID, name, data)
			if err != nil {
				c.InternalServerError("failed to update collection")
				return
			}
			response = dto.UpsertCollectionResponse{
				ID:          updated.ID,
				WorkspaceID: updated.WorkspaceID,
				Name:        updated.Name,
				Version:     updated.Version,
				Created:     false,
			}
			_ = c.JSON(200, response)
			return
		}
	}

	// Collection doesn't exist — create
	if name == "" {
		c.BadRequest("name is required when creating a new collection")
		return
	}

	newCollection, err := h.collectionService.Create(ctx, workspaceID, name, data, uuid.Nil)
	if err != nil {
		c.InternalServerError("failed to create collection")
		return
	}

	response = dto.UpsertCollectionResponse{
		ID:          newCollection.ID,
		WorkspaceID: newCollection.WorkspaceID,
		Name:        newCollection.Name,
		Version:     newCollection.Version,
		Created:     true,
	}
	_ = c.JSON(201, response)
}
