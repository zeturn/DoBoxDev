package handlers

import (
	"context"
	"docode/internal/database"
	"docode/internal/docker"
	"docode/internal/middleware"
	"docode/internal/models"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type NetworkHandler struct {
	dockerService *docker.DockerService
}

func NewNetworkHandler(dockerService *docker.DockerService) *NetworkHandler {
	return &NetworkHandler{dockerService: dockerService}
}

func (h *NetworkHandler) CreateNetwork(c *fiber.Ctx) error {
	var req struct {
		Name       string `json:"name"`
		Driver     string `json:"driver"`
		Attachable bool   `json:"attachable"`
		Internal   bool   `json:"internal"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	userID := middleware.GetUserID(c)
	id, err := h.dockerService.CreateNetwork(context.Background(), req.Name, req.Driver, req.Attachable, req.Internal)
	if err != nil {
		writeAudit(userID, 0, "network_create", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	writeAudit(userID, 0, "network_create", "success", req.Name)
	return c.JSON(fiber.Map{"id": id, "message": "Network created"})
}

func (h *NetworkHandler) DeleteNetwork(c *fiber.Ctx) error {
	networkID := c.Params("id")
	if strings.TrimSpace(networkID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}
	userID := middleware.GetUserID(c)
	if err := h.dockerService.RemoveNetwork(context.Background(), networkID); err != nil {
		writeAudit(userID, 0, "network_delete", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	writeAudit(userID, 0, "network_delete", "success", networkID)
	return c.JSON(fiber.Map{"message": "Network removed"})
}

func (h *NetworkHandler) ListNetworks(c *fiber.Ctx) error {
	items, err := h.dockerService.ListNetworks(context.Background())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(items)
}

func (h *NetworkHandler) InspectNetwork(c *fiber.Ctx) error {
	networkID := c.Params("id")
	if strings.TrimSpace(networkID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}
	info, err := h.dockerService.InspectNetwork(context.Background(), networkID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info.Network)
}

func (h *NetworkHandler) ConnectContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	networkID := c.Params("id")
	var req struct {
		ContainerDBID string `json:"container_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.ContainerDBID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "container_id is required"})
	}
	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", req.ContainerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Container not found"})
	}
	if err := h.dockerService.ConnectContainerToNetwork(context.Background(), networkID, container.ContainerID); err != nil {
		writeAudit(userID, container.ID, "network_connect", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	writeAudit(userID, container.ID, "network_connect", "success", networkID)
	return c.JSON(fiber.Map{"message": "Container connected to network"})
}

func (h *NetworkHandler) DisconnectContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	networkID := c.Params("id")
	var req struct {
		ContainerDBID string `json:"container_id"`
		Force         bool   `json:"force"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.ContainerDBID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "container_id is required"})
	}
	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", req.ContainerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Container not found"})
	}
	if err := h.dockerService.DisconnectContainerFromNetwork(context.Background(), networkID, container.ContainerID, req.Force); err != nil {
		writeAudit(userID, container.ID, "network_disconnect", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	writeAudit(userID, container.ID, "network_disconnect", "success", networkID)
	return c.JSON(fiber.Map{"message": "Container disconnected from network"})
}
