package middleware

import (
	"net/http"

	"github.com/leonelquinteros/gotext"

	"github.com/coder-lulu/nb-agent/internal/app"
)

// Status 检查程序状态
func Status(t *gotext.Locale) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch app.Status {
			case app.StatusUpgrade:
				Abort(w, http.StatusServiceUnavailable, t.Get("panel is upgrading, please refresh later"))
				return
			case app.StatusMaintain:
				Abort(w, http.StatusServiceUnavailable, t.Get("panel is maintaining, please refresh later"))
				return
			case app.StatusClosed:
				Abort(w, http.StatusServiceUnavailable, t.Get("panel is closed"))
				return
			case app.StatusFailed:
				Abort(w, http.StatusInternalServerError, t.Get("panel run error, please check or contact support"))
				return
			default:
				next.ServeHTTP(w, r)
			}
		})
	}
}
