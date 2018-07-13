// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package beater

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/logp"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	glob "github.com/ryanuber/go-glob"
	uuid "github.com/satori/go.uuid"
	"golang.org/x/time/rate"
)

var (
	// Route types define how to with specifics for a type of route
	FrontendRouteType = v1RouteType{
		"FrontendRouteType",
		frontendHandler,
		func(beaterConfig *Config, rd decoder.ReqDecoder) decoder.ReqDecoder {
			return decoder.DecodeUserData(rd, beaterConfig.AugmentEnabled)
		},
		sourcemappingConfig,
	}

	BackendRouteType = v1RouteType{
		"BackendRouteType",
		backendHandler,
		func(beaterConfig *Config, rd decoder.ReqDecoder) decoder.ReqDecoder {
			return decoder.DecodeSystemData(rd, beaterConfig.AugmentEnabled)
		},
		nil,
	}

	MetricsRouteType = v1RouteType{
		"MetricsRouteType",
		metricsHandler,
		func(beaterConfig *Config, rd decoder.ReqDecoder) decoder.ReqDecoder {
			return decoder.DecodeSystemData(rd, beaterConfig.AugmentEnabled)
		},
		nil,
	}

	SourcemapRouteType = v1RouteType{
		"SourcemapRouteType",
		sourcemapUploadHandler,
		func(*Config, decoder.ReqDecoder) decoder.ReqDecoder { return decoder.DecodeSourcemapFormData },
		sourcemappingConfig,
	}
)

func concurrencyLimitHandler(beaterConfig *Config, h http.Handler) http.Handler {
	semaphore := make(chan struct{}, beaterConfig.ConcurrentRequests)
	release := func() {
		<-semaphore
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := time.Now()
		var wait = func() int64 {
			return time.Now().Sub(t).Nanoseconds() / 1e6
		}
		select {
		case semaphore <- struct{}{}:
			concurrentWait.Add(wait())
			defer release()
			h.ServeHTTP(w, r)
		case <-time.After(beaterConfig.MaxRequestQueueTime):
			concurrentWait.Add(wait())
			sendStatus(w, r, tooManyConcurrentRequestsResponse)
		}

	})
}

func backendHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		concurrencyLimitHandler(beaterConfig,
			authHandler(beaterConfig.SecretToken, h)))
}

func frontendHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.Frontend.isEnabled(),
			concurrencyLimitHandler(beaterConfig,
				ipRateLimitHandler(beaterConfig.Frontend.RateLimit,
					corsHandler(beaterConfig.Frontend.AllowOrigins, h)))))
}

func metricsHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.Metrics.isEnabled(),
			authHandler(beaterConfig.SecretToken, h)))
}

func sourcemapUploadHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.Frontend.isEnabled(),
			authHandler(beaterConfig.SecretToken, h)))
}

func healthCheckHandler() http.Handler {
	return logHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sendStatus(w, r, okResponse)
		}))
}

type logContextKey string

var reqLoggerContextKey = logContextKey("requestLogger")

func logHandler(h http.Handler) http.Handler {
	logger := logp.NewLogger("request")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := uuid.NewV4()

		requestCounter.Inc()

		reqLogger := logger.With(
			"request_id", reqID,
			"method", r.Method,
			"URL", r.URL,
			"content_length", r.ContentLength,
			"remote_address", utility.RemoteAddr(r),
			"user-agent", r.Header.Get("User-Agent"))

		lr := r.WithContext(
			context.WithValue(r.Context(), reqLoggerContextKey, reqLogger),
		)

		lw := utility.NewRecordingResponseWriter(w)

		h.ServeHTTP(lw, lr)

		if lw.Code <= 399 {
			reqLogger.Infow("handled request", []interface{}{"response_code", lw.Code}...)
		}
	})
}

func killSwitchHandler(killSwitch bool, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if killSwitch {
			h.ServeHTTP(w, r)
		} else {
			sendStatus(w, r, forbiddenResponse(errors.New("endpoint is disabled")))
		}
	})
}

func ipRateLimitHandler(rateLimit int, h http.Handler) http.Handler {
	cache, _ := lru.New(rateLimitCacheSize)

	var deny = func(ip string) bool {
		if !cache.Contains(ip) {
			cache.Add(ip, rate.NewLimiter(rate.Limit(rateLimit), rateLimit*rateLimitBurstMultiplier))
		}
		var limiter, _ = cache.Get(ip)
		return !limiter.(*rate.Limiter).Allow()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deny(utility.RemoteAddr(r)) {
			sendStatus(w, r, rateLimitedResponse)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func authHandler(secretToken string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthorized(r, secretToken) {
			sendStatus(w, r, unauthorizedResponse)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// isAuthorized checks the Authorization header. It must be in the form of:
//   Authorization: Bearer <secret-token>
// Bearer must be part of it.
func isAuthorized(req *http.Request, secretToken string) bool {
	// No token configured
	if secretToken == "" {
		return true
	}
	header := req.Header.Get("Authorization")
	parts := strings.Split(header, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(parts[1]), []byte(secretToken)) == 1
}

func corsHandler(allowedOrigins []string, h http.Handler) http.Handler {

	var isAllowed = func(origin string) bool {
		for _, allowed := range allowedOrigins {
			if glob.Glob(allowed, origin) {
				return true
			}
		}
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// origin header is always set by the browser
		origin := r.Header.Get("Origin")
		validOrigin := isAllowed(origin)

		if r.Method == "OPTIONS" {

			// setting the ACAO header is the way to tell the browser to go ahead with the request
			if validOrigin {
				// do not set the configured origin(s), echo the received origin instead
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}

			// tell browsers to cache response requestHeaders for up to 1 hour (browsers might ignore this)
			w.Header().Set("Access-Control-Max-Age", "3600")
			// origin must be part of the cache key so that we can handle multiple allowed origins
			w.Header().Set("Vary", "Origin")

			// required if Access-Control-Request-Method and Access-Control-Request-Headers are in the requestHeaders
			w.Header().Set("Access-Control-Allow-Methods", supportedMethods)
			w.Header().Set("Access-Control-Allow-Headers", supportedHeaders)

			w.Header().Set("Content-Length", "0")

			sendStatus(w, r, okResponse)

		} else if validOrigin {
			// we need to check the origin and set the ACAO header in both the OPTIONS preflight and the actual request
			w.Header().Set("Access-Control-Allow-Origin", origin)
			h.ServeHTTP(w, r)

		} else {
			sendStatus(w, r, forbiddenResponse(errors.New("origin: '"+origin+"' is not allowed")))
		}
	})
}
