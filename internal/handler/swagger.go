package handler

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"gopkg.in/yaml.v3"
)

//go:embed openapi.yaml
var openAPISpecYAML []byte

//go:embed web/swagger-ui/*
var swaggerUIFiles embed.FS

var openAPISpecJSON []byte

func init() {
	var spec any
	if err := yaml.Unmarshal(openAPISpecYAML, &spec); err != nil {
		panic("openapi.yaml inválido: " + err.Error())
	}
	data, err := json.Marshal(spec)
	if err != nil {
		panic("openapi json: " + err.Error())
	}
	openAPISpecJSON = data
}

func RegisterSwagger(e *echo.Echo) {
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusTemporaryRedirect, "/swagger")
	})
	e.GET("/swagger", serveSwaggerUI)
	e.GET("/swagger/", serveSwaggerUI)
	e.GET("/openapi.yaml", serveOpenAPIYAML)
	e.GET("/openapi.json", serveOpenAPIJSON)

	ui, _ := fs.Sub(swaggerUIFiles, "web/swagger-ui")
	e.GET("/swagger-ui/*", echo.WrapHandler(http.StripPrefix("/swagger-ui/", http.FileServer(http.FS(ui)))))
}

func serveOpenAPIYAML(c echo.Context) error {
	return c.Blob(http.StatusOK, "application/yaml", openAPISpecYAML)
}

func serveOpenAPIJSON(c echo.Context) error {
	return c.Blob(http.StatusOK, "application/json", openAPISpecJSON)
}

func serveSwaggerUI(c echo.Context) error {
	html := strings.Replace(swaggerUIHTML, "__OPENAPI_SPEC__", string(openAPISpecJSON), 1)
	return c.HTML(http.StatusOK, html)
}

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="pt-BR">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Atlas Knowledge API — Swagger</title>
  <link rel="stylesheet" href="/swagger-ui/swagger-ui.css" />
  <style>
    body { margin: 0; background: #fafafa; }
    #boot-error {
      display: none; padding: 1.5rem; margin: 2rem auto; max-width: 560px;
      font-family: system-ui, sans-serif; background: #fff3f3; border: 1px solid #f5c2c2;
      border-radius: 8px; color: #842029;
    }
  </style>
</head>
<body>
  <div id="boot-error"></div>
  <div id="swagger-ui"></div>
  <script src="/swagger-ui/swagger-ui-bundle.js"></script>
  <script>
    (function () {
      function showError(msg) {
        var el = document.getElementById('boot-error');
        el.style.display = 'block';
        el.innerHTML = '<strong>Swagger não carregou.</strong><p>' + msg + '</p>' +
          '<p>Spec JSON: <a href="/openapi.json">/openapi.json</a></p>';
      }
      if (typeof SwaggerUIBundle === 'undefined') {
        showError('Arquivo swagger-ui-bundle.js não encontrado. Reinicie a API com <code>go run ./cmd/api</code>.');
        return;
      }
      try {
        SwaggerUIBundle({
          spec: __OPENAPI_SPEC__,
          dom_id: '#swagger-ui',
          deepLinking: true,
          persistAuthorization: true,
        });
      } catch (e) {
        showError(e.message);
      }
    })();
  </script>
</body>
</html>`
