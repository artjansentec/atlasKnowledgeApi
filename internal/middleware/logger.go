package middleware

import (
	"fmt"
	"os"
	"time"

	"github.com/labstack/echo/v4"
)

func wantColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("FORCE_COLOR") != "" || os.Getenv("CLICOLOR_FORCE") != "" {
		return true
	}
	return os.Getenv("TERM") != "dumb"
}

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiCyan    = "\033[36m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiRed     = "\033[31m"
	ansiMagenta = "\033[35m"
	ansiBlue    = "\033[34m"
	ansiWhite   = "\033[37m"
)

// ColoredLogger imprime requests HTTP com cores por status e método.
func ColoredLogger() echo.MiddlewareFunc {
	useColor := wantColor()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()
			status := res.Status
			if status == 0 {
				status = 200
			}
			latency := time.Since(start)
			method := req.Method
			uri := req.RequestURI
			if uri == "" {
				uri = req.URL.RequestURI()
			}

			if !useColor {
				fmt.Printf("%s | %3d | %10s | %-7s %s\n",
					start.Format("15:04:05"), status, latency.Round(time.Microsecond), method, uri)
				return nil
			}

			fmt.Printf("%s%s%s │ %s%3d%s │ %s%10s%s │ %s%-7s%s %s%s%s\n",
				ansiDim, start.Format("15:04:05"), ansiReset,
				statusColor(status), status, ansiReset,
				ansiDim, latency.Round(time.Microsecond), ansiReset,
				methodColor(method), method, ansiReset,
				ansiWhite, uri, ansiReset,
			)
			return nil
		}
	}
}

func statusColor(status int) string {
	switch {
	case status >= 500:
		return ansiBold + ansiRed
	case status >= 400:
		return ansiBold + ansiYellow
	case status >= 300:
		return ansiCyan
	case status >= 200:
		return ansiBold + ansiGreen
	default:
		return ansiWhite
	}
}

func methodColor(method string) string {
	switch method {
	case "GET":
		return ansiBlue
	case "POST":
		return ansiGreen
	case "PUT", "PATCH":
		return ansiYellow
	case "DELETE":
		return ansiRed
	case "OPTIONS":
		return ansiDim + ansiMagenta
	default:
		return ansiWhite
	}
}
