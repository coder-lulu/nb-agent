package middleware

import (
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/libtnb/sessions"
	"github.com/coder-lulu/nb-agent/pkg/punycode"
)

// Entrance 确保通过正确的入口访问
func Entrance(t *gotext.Locale, conf *koanf.Koanf, session *sessions.Manager) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, err := session.GetSession(r)
			if err != nil {
				Abort(w, http.StatusInternalServerError, "%v", err)
				return
			}

			entrance := strings.TrimSuffix(conf.String("http.entrance"), "/")
			if !strings.HasPrefix(entrance, "/") {
				entrance = "/" + entrance
			}

			// 情况一：设置了绑定域名、IP、UA，且请求不符合要求，返回错误
			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}
			if strings.Contains(host, "xn--") {
				if decoded, err := punycode.DecodeDomain(host); err == nil {
					host = decoded
				}
			}
			if len(conf.Strings("http.bind_domain")) > 0 && !slices.Contains(conf.Strings("http.bind_domain"), host) {
				Abort(w, http.StatusTeapot, t.Get("invalid request domain: %s", r.Host))
				return
			}
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if len(conf.Strings("http.bind_ip")) > 0 {
				allowed := false
				requestIP := net.ParseIP(ip)
				if requestIP != nil {
					for _, allowedIP := range conf.Strings("http.bind_ip") {
						if strings.Contains(allowedIP, "/") {
							// CIDR
							if _, ipNet, err := net.ParseCIDR(allowedIP); err == nil && ipNet.Contains(requestIP) {
								allowed = true
								break
							}
						} else {
							// IP
							if allowedIP == ip {
								allowed = true
								break
							}
						}
					}
				}
				if !allowed {
					Abort(w, http.StatusTeapot, t.Get("invalid request ip: %s", ip))
					return
				}
			}
			if len(conf.Strings("http.bind_ua")) > 0 && !slices.Contains(conf.Strings("http.bind_ua"), r.UserAgent()) {
				Abort(w, http.StatusTeapot, t.Get("invalid request user agent: %s", r.UserAgent()))
				return
			}

			// 情况二：请求路径与入口路径相同或未设置访问入口，标记通过验证并重定向
			if (strings.TrimSuffix(r.URL.Path, "/") == entrance || entrance == "/") &&
				r.Header.Get("Authorization") == "" {
				sess.Put("verify_entrance", true)
				// 设置入口的情况下进行重定向
				if entrance != "/" {
					render := chix.NewRender(w, r)
					defer render.Release()
					render.Redirect("/login")
					return
				}
			}

			// 情况三：通过APIKey+入口路径访问，重写请求路径并跳过验证
			if strings.HasPrefix(r.URL.Path, entrance) && r.Header.Get("Authorization") != "" {
				// 只在设置了入口路径的情况下，才进行重写
				if entrance != "/" {
					if rctx := chi.RouteContext(r.Context()); rctx != nil {
						rctx.RoutePath = strings.TrimPrefix(rctx.RoutePath, entrance)
						r.URL.Path = strings.TrimPrefix(r.URL.Path, entrance)
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			// 情况四：非调试模式且未通过验证的请求，返回错误
			if !conf.Bool("app.debug") &&
				sess.Missing("verify_entrance") &&
				r.URL.Path != "/robots.txt" {
				Abort(w, http.StatusTeapot, t.Get("invalid access entrance"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
