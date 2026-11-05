package docs

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.json
var OpenAPISpec []byte

// RegisterDocRoutes registers interactive documentation endpoints onto the Gin router.
func RegisterDocRoutes(router *gin.Engine) {
	serveOpenAPI := func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", OpenAPISpec)
	}

	scalarHTML := `<!doctype html>
<html>
  <head>
    <title>Ticketing Platform API Docs</title>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" type="image/svg+xml" href="https://scalar.com/favicon.svg" />
  </head>
  <body>
    <script
      id="api-reference"
      data-url="/openapi.json"
      data-configuration='{"theme":"purple","layout":"modern"}'></script>
    <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
  </body>
</html>`

	serveScalar := func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(scalarHTML))
	}

	swaggerHTML := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Swagger UI - Ticketing API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin="anonymous"></script>
<script>
  window.onload = () => {
    window.ui = SwaggerUIBundle({
      url: '/openapi.json',
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [
        SwaggerUIBundle.presets.apis,
        SwaggerUIBundle.SwaggerUIStandalonePreset
      ],
      layout: "BaseLayout"
    });
  };
</script>
</body>
</html>`

	serveSwagger := func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
	}

	// 1. Top-level Documentation Endpoints
	router.GET("/openapi.json", serveOpenAPI)
	router.GET("/docs", serveScalar)
	router.GET("/docs/", serveScalar)
	router.GET("/swagger", serveSwagger)
	router.GET("/swagger/", serveSwagger)

	// 2. Also register under /api/v1 prefix for frontend convenience
	v1Docs := router.Group("/api/v1")
	{
		v1Docs.GET("/openapi.json", serveOpenAPI)
		v1Docs.GET("/docs", serveScalar)
		v1Docs.GET("/docs/", serveScalar)
		v1Docs.GET("/swagger", serveSwagger)
		v1Docs.GET("/swagger/", serveSwagger)
	}
}
