package api

import (
	"fmt"
	"net/http"
	"time"

	"ocf-worker/internal/validation"

	"github.com/gin-gonic/gin"
)

// ValidationMiddleware injecte l'APIValidator dans le contexte
func ValidationMiddleware(validator *validation.APIValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Injecter le validator dans le contexte
		c.Set("validator", validator)
		c.Next()
	}
}

// ValidationErrorLogger middleware pour logger les erreurs de validation
func ValidationErrorLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Logger spécifiquement les erreurs de validation (400)
		if param.StatusCode == http.StatusBadRequest {
			return fmt.Sprintf("[VALIDATION] %v | %3d | %13v | %15s | %-7s %#v\n",
				param.TimeStamp.Format("2006/01/02 - 15:04:05"),
				param.StatusCode,
				param.Latency,
				param.ClientIP,
				param.Method,
				param.Path,
			)
		}
		return ""
	})
}

// StandardErrorResponse middleware pour standardiser les réponses d'erreur
func StandardErrorResponse() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Si c'est une erreur et qu'aucune réponse n'a été envoyée
		if c.Writer.Status() >= 400 && !c.Writer.Written() {
			c.JSON(c.Writer.Status(), gin.H{
				"error":     "An error occurred",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"path":      c.Request.URL.Path,
			})
		}
	}
}

// RateLimitMiddleware - Rate limiting basique par IP
func RateLimitMiddleware(requestsPerMinute int) gin.HandlerFunc {
	// Map simple pour tracking (en production, utiliser Redis)
	clients := make(map[string][]time.Time)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		// Nettoyer les anciens timestamps (> 1 minute)
		if timestamps, exists := clients[clientIP]; exists {
			var validTimestamps []time.Time
			for _, timestamp := range timestamps {
				if now.Sub(timestamp) < time.Minute {
					validTimestamps = append(validTimestamps, timestamp)
				}
			}
			clients[clientIP] = validTimestamps
		}

		// Vérifier le nombre de requêtes
		if len(clients[clientIP]) >= requestsPerMinute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": "60 seconds",
			})
			c.Abort()
			return
		}

		// Ajouter la requête actuelle
		clients[clientIP] = append(clients[clientIP], now)
		c.Next()
	}
}

// SecurityHeadersMiddleware ajoute des headers de sécurité
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// CORS plus restrictif en production
		if gin.Mode() == gin.ReleaseMode {
			// En production, configurer les domaines autorisés
			c.Header("Access-Control-Allow-Origin", "https://your-domain.com")
		} else {
			// En développement, plus permissif
			c.Header("Access-Control-Allow-Origin", "*")
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// GetValidator helper pour récupérer le validator du contexte
func GetValidator(c *gin.Context) *validation.APIValidator {
	if validator, exists := c.Get("validator"); exists {
		if apiValidator, ok := validator.(*validation.APIValidator); ok {
			return apiValidator
		}
	}
	return nil
}
