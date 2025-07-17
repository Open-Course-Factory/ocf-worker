// internal/api/swagger.go
package api

import (
	"ocf-worker/docs"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupSwagger configure les routes Swagger
func SetupSwagger(router *gin.Engine) {
	env := os.Getenv("ENVIRONMENT")
	if env == "development" || env == "test" {
		docs.SwaggerInfo.Schemes = []string{"http", "https"}
	} else {
		docs.SwaggerInfo.Schemes = []string{"https"}
	}

	// Configuration Swagger UI avec thème personnalisé
	// config := ginSwagger.Config{
	// 	URL:                      "/swagger/doc.json",
	// 	DocExpansion:             "list",
	// 	DomID:                    "#swagger-ui",
	// 	InstanceName:             "swagger",
	// 	Title:                    "OCF Worker API Documentation",
	// 	DefaultModelsExpandDepth: 1,
	// 	DefaultModelExpandDepth:  1,
	// 	DefaultModelRendering:    "example",
	// 	DisplayRequestDuration:   true,
	// 	MaxDisplayedTags:         10,
	// 	ShowExtensions:           true,
	// 	ShowCommonExtensions:     true,
	// 	DeepLinking:              true,
	// }

	// Route principale Swagger UI
	router.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.URL("doc.json"),
		ginSwagger.DocExpansion("list"),
		ginSwagger.DeepLinking(true),
		ginSwagger.DefaultModelsExpandDepth(1),
		ginSwagger.InstanceName("swagger"),
	))

	// // Route de redirection depuis la racine
	// router.GET("/docs", func(c *gin.Context) {
	// 	c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	// })

	// // Route alternative
	// router.GET("/api-docs", func(c *gin.Context) {
	// 	c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	// })

	// // Endpoint pour le JSON brut
	// router.GET("/swagger.json", func(c *gin.Context) {
	// 	c.Redirect(http.StatusMovedPermanently, "/swagger/doc.json")
	// })
}

// SwaggerInfo contient les métadonnées pour Swagger
type SwaggerInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Host        string `json:"host"`
	BasePath    string `json:"basePath"`
}

// GetSwaggerInfo retourne les informations Swagger
func GetSwaggerInfo() SwaggerInfo {
	return SwaggerInfo{
		Title:       "OCF Worker API",
		Description: "API complète pour la génération de cours OCF",
		Version:     "2.0.0",
		Host:        "localhost:8081",
		BasePath:    "/api/v1",
	}
}
