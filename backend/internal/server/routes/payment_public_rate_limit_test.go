package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestPublicOrderVerificationRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "bounded-and-expiring", true: "redis-outage"}[unavailable], func(t *testing.T) {
			mr := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr(), MaxRetries: -1})
			t.Cleanup(func() { _ = rdb.Close() })
			if unavailable {
				mr.Close()
			}
			router := gin.New()
			require.NoError(t, router.SetTrustedProxies(nil))
			router.Use(servermiddleware.SessionBindingContext(&config.Config{}))
			noop := func(c *gin.Context) { c.Next() }
			RegisterPaymentRoutes(router.Group("/api/v1"), &handler.PaymentHandler{}, &handler.PaymentWebhookHandler{}, &admin.PaymentHandler{}, servermiddleware.JWTAuthMiddleware(noop), servermiddleware.AdminAuthMiddleware(noop), servermiddleware.AuditLogMiddleware(noop), nil, nil, rdb)
			post := func(path, remote string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Forwarded-For", "203.0.113.99")
				req.RemoteAddr = remote
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				return response
			}
			path := "/api/v1/payment/public/orders/verify"
			for i := 0; i < 20; i++ {
				require.Equal(t, http.StatusBadRequest, post(path, "198.51.100.20:1234").Code)
			}
			response := post(path, "198.51.100.20:4321")
			if unavailable {
				require.Equal(t, http.StatusBadRequest, response.Code, "preserve fail-open payment-result lookup")
				return
			}
			require.Equal(t, http.StatusTooManyRequests, response.Code)
			require.NotEmpty(t, response.Header().Get("Retry-After"))
			require.Equal(t, http.StatusBadRequest, post(path, "198.51.100.21:1234").Code, "shared untrusted forwarded IP must not merge real IP budgets")
			require.Equal(t, http.StatusBadRequest, post("/api/v1/payment/public/orders/resolve", "198.51.100.20:1234").Code, "signed resume flow remains available")
			mr.FastForward(time.Minute)
			require.Equal(t, http.StatusBadRequest, post(path, "198.51.100.20:1234").Code)
		})
	}
}

func TestPublicOrderVerificationRetainsConfiguredForwardedIPPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	cfg := &config.Config{}
	cfg.SetForwardedClientIPSettings(true, nil)
	router.Use(servermiddleware.SessionBindingContext(cfg))
	noop := func(c *gin.Context) { c.Next() }
	RegisterPaymentRoutes(router.Group("/api/v1"), &handler.PaymentHandler{}, &handler.PaymentWebhookHandler{}, &admin.PaymentHandler{}, servermiddleware.JWTAuthMiddleware(noop), servermiddleware.AdminAuthMiddleware(noop), servermiddleware.AuditLogMiddleware(noop), nil, nil, rdb)
	for i := 0; i < 22; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/payment/public/orders/verify", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1))
		req.RemoteAddr = "198.51.100.20:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.Equal(t, http.StatusBadRequest, response.Code, "compatibility mode deliberately uses the forwarded-IP budget, not a hard peer-IP cap")
	}
}
