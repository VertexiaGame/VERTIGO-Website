package handlers

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"
	"vertexia-frontend/backend/renderer"
)

var createCategoryNames = map[string]string{
	"hat":    "Hat",
	"face":   "Face",
	"shirt":  "Shirt",
	"tshirt": "T-Shirt",
	"pants":  "Pants",
	"tool":   "Tool",
}

func normalizeCreateType(value string) string {
	switch value {
	case "game", "shop", "asset":
		return value
	default:
		return "shop"
	}
}

func normalizeCreateCategory(value string) (string, string) {
	if name, ok := createCategoryNames[value]; ok {
		return value, name
	}
	return "hat", "Hat"
}

func CreateGet(c fiber.Ctx) error {
	if username := GetActiveUser(c); username == "" {
		return c.Redirect().To("/login")
	}

	activeType := normalizeCreateType(c.Query("type", "shop"))
	activeCat, activeCatName := normalizeCreateCategory(c.Query("cat", "hat"))

	return Render(c, "pages/create", fiber.Map{
		"Title":         "Create - VERTEXIA",
		"ActiveType":    activeType,
		"ActiveCat":     activeCat,
		"ActiveCatName": activeCatName,
	}, "layouts/main")
}

func CreatePreviewPost(c fiber.Ctx) error {
	if username := GetActiveUser(c); username == "" {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
	}

	category := c.FormValue("category", "")
	if _, ok := createCategoryNames[category]; !ok {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid category")
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Missing file")
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	isModel := category == "hat" || category == "face" || category == "tool"
	if isModel && ext != ".obj" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid model file")
	}
	if !isModel && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid image file")
	}

	src, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Could not read upload")
	}
	defer src.Close()

	tmpFile, err := os.CreateTemp("", "crtpreview*"+ext)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Could not save upload")
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		return c.Status(fiber.StatusInternalServerError).SendString("Could not save upload")
	}
	tmpFile.Close()

	var previewTexture string
	if isModel {
		if texHeader, err := c.FormFile("texture"); err == nil {
			texExt := strings.ToLower(filepath.Ext(texHeader.Filename))
			if texExt == ".png" || texExt == ".jpg" || texExt == ".jpeg" {
				texSrc, err := texHeader.Open()
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).SendString("Could not read texture")
				}
				tmpTex, err := os.CreateTemp("", "crtpreviewtex*"+texExt)
				if err != nil {
					texSrc.Close()
					return c.Status(fiber.StatusInternalServerError).SendString("Could not save texture")
				}
				texTmpPath := tmpTex.Name()
				defer os.Remove(texTmpPath)
				if _, err := io.Copy(tmpTex, texSrc); err != nil {
					tmpTex.Close()
					texSrc.Close()
					return c.Status(fiber.StatusInternalServerError).SendString("Could not save texture")
				}
				tmpTex.Close()
				texSrc.Close()
				previewTexture = texTmpPath
			}
		}
	}

	imgBytes, err := renderer.RenderCreatePreview(category, tmpPath, previewTexture)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	c.Set("Content-Type", "image/png")
	return c.Send(imgBytes)
}