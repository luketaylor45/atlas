package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
)

// GetNews returns all news items
func GetNews(c *gin.Context) {
	var news []models.News
	if err := database.DB.Order("created_at DESC").Find(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch news"})
		return
	}
	c.JSON(http.StatusOK, news)
}

// CreateNews adds a new news item (Admin only)
func CreateNews(c *gin.Context) {
	var req struct {
		Title   string `json:"title" binding:"required"`
		Content string `json:"content" binding:"required"`
		Type    string `json:"type" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	news := models.News{
		Title:   req.Title,
		Content: req.Content,
		Type:    req.Type,
	}

	if err := database.DB.Create(&news).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create news item"})
		return
	}

	c.JSON(http.StatusCreated, news)
}

// UpdateNews updates an existing news item (Admin only)
func UpdateNews(c *gin.Context) {
	id := c.Param("id")
	var news models.News
	if err := database.DB.First(&news, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "News item not found"})
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Type    string `json:"type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != "" {
		news.Title = req.Title
	}
	if req.Content != "" {
		news.Content = req.Content
	}
	if req.Type != "" {
		news.Type = req.Type
	}

	database.DB.Save(&news)
	c.JSON(http.StatusOK, news)
}

// DeleteNews removes a news item (Admin only)
func DeleteNews(c *gin.Context) {
	id := c.Param("id")
	if err := database.DB.Delete(&models.News{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete news item"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
