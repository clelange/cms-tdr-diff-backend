package main

import (
	"github.com/gin-gonic/gin"
)

// User struct
type User struct {
	UserName string
}

func respondWithError(c *gin.Context, code int, message interface{}) {
	c.AbortWithStatusJSON(code, gin.H{"error": message})
}

func tokenAuthMiddleware(apiToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Request.Header.Get("api_token")

		if token == "" {
			respondWithError(c, 401, "API token required")
			return
		}

		if token != apiToken {
			respondWithError(c, 401, "Invalid API token")
			return
		}

		c.Next()
	}
}
