package handlers

import (
	"context"
	"encoding/json"
	"errors"
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
	if err := c.BindJSON(&req); err != nil {
		c.BadRequest("invalid request body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		c.BadRequest("name is required")
		return
	}

	if len(req.Spec) == 0 {
		c.BadRequest("spec is required")
		return
	}

	// Parse the OpenAPI spec (auto-detects JSON/YAML)
	var specBytes []byte

	// Check if spec is a string (YAML) or JSON object
	var yamlStr string
	if err := json.Unmarshal(req.Spec, &yamlStr); err == nil {
		// It's a string, treat as YAML
		specBytes = []byte(yamlStr)
	} else {
		// It's already JSON
		specBytes = req.Spec
	}

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

	// Check if collection already exists by name
	existing, err := h.collectionService.GetByWorkspaceAndName(context.Background(), workspaceID, req.Name)

	if err != nil && !errors.Is(err, services.ErrCollectionNotFound) {
		c.InternalServerError("failed to check existing collection")
		return
	}

	var collection interface {
		GetID() uuid.UUID
		GetWorkspaceID() uuid.UUID
		GetName() string
		GetVersion() int
	}
	var created bool

	if existing != nil {
		// Update existing collection
		if req.Force {
			// Force update bypasses version check
			updated, err := h.collectionService.ForceUpdate(context.Background(), existing.ID, req.Name, data)
			if err != nil {
				c.InternalServerError("failed to update collection")
				return
			}
			collection = &collectionWrapper{updated.ID, updated.WorkspaceID, updated.Name, updated.Version}
		} else {
			// Normal update with version check
			updated, err := h.collectionService.Update(context.Background(), existing.ID, &req.Name, data, existing.Version, uuid.Nil)
			if err != nil {
				if errors.Is(err, services.ErrVersionConflict) {
					c.JSON(409, map[string]string{
						"error":   "version conflict",
						"message": "collection has been modified, use force=true to override",
					})
					return
				}
				c.InternalServerError("failed to update collection")
				return
			}
			collection = &collectionWrapper{updated.ID, updated.WorkspaceID, updated.Name, updated.Version}
		}
		created = false
	} else {
		// Create new collection
		newCollection, err := h.collectionService.Create(context.Background(), workspaceID, req.Name, data, uuid.Nil)
		if err != nil {
			c.InternalServerError("failed to create collection")
			return
		}
		collection = &collectionWrapper{newCollection.ID, newCollection.WorkspaceID, newCollection.Name, newCollection.Version}
		created = true
	}

	response := dto.UpsertCollectionResponse{
		ID:          collection.GetID(),
		WorkspaceID: collection.GetWorkspaceID(),
		Name:        collection.GetName(),
		Version:     collection.GetVersion(),
		Created:     created,
	}

	if created {
		_ = c.JSON(201, response)
	} else {
		_ = c.JSON(200, response)
	}
}

// collectionWrapper wraps collection fields to implement the interface
type collectionWrapper struct {
	id          uuid.UUID
	workspaceID uuid.UUID
	name        string
	version     int
}

func (w *collectionWrapper) GetID() uuid.UUID          { return w.id }
func (w *collectionWrapper) GetWorkspaceID() uuid.UUID { return w.workspaceID }
func (w *collectionWrapper) GetName() string           { return w.name }
func (w *collectionWrapper) GetVersion() int           { return w.version }
