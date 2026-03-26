package handlers

import (
	"docode/internal/database"
	"docode/internal/middleware"
	"docode/internal/models"

	"github.com/gofiber/fiber/v2"
)

func writeAudit(userID uint, containerID uint, action string, status string, detail string) {
	_ = database.DB.Create(&models.OperationAudit{
		UserID:      userID,
		ContainerID: containerID,
		Action:      action,
		Status:      status,
		Detail:      detail,
	}).Error
}

func (h *ContainerHandler) ListAudits(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	var audits []models.OperationAudit
	if err := database.DB.Where("user_id = ? AND container_id = ?", userID, container.ID).Order("id desc").Limit(200).Find(&audits).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch audits",
		})
	}

	return c.JSON(audits)
}

func (h *ContainerHandler) ListUserAudits(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	scope := c.Query("scope", "all")

	query := database.DB.Where("user_id = ?", userID)
	if scope == "infra" {
		query = query.Where("container_id = ?", 0)
	}
	if scope == "container" {
		query = query.Where("container_id <> ?", 0)
	}

	var audits []models.OperationAudit
	if err := query.Order("id desc").Limit(300).Find(&audits).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch audits",
		})
	}
	return c.JSON(audits)
}
