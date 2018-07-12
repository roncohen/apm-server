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
	"encoding/json"
	"expvar"
	"log"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
)

const (
	BackendTransactionsURL  = "/v1/transactions"
	FrontendTransactionsURL = "/v1/client-side/transactions"
	BackendErrorsURL        = "/v1/errors"
	FrontendErrorsURL       = "/v1/client-side/errors"
	HealthCheckURL          = "/healthcheck"
	MetricsURL              = "/v1/metrics"
	SourcemapsURL           = "/v1/client-side/sourcemaps"

	V2BackendURL  = "/v2/intake"
	V2FrontendURL = "/v2/client-side/intake"

	rateLimitCacheSize       = 1000
	rateLimitBurstMultiplier = 2

	supportedHeaders = "Content-Type, Content-Encoding, Accept"
	supportedMethods = "POST, OPTIONS"
)

type ReportingHandlerFactory func(*Config, reporter) http.Handler

type serverResponse struct {
	err     error
	code    int
	counter *monitoring.Int
}

func (s serverResponse) IsError() bool {
	return s.code >= 400
}

var (
	serverMetrics = monitoring.Default.NewRegistry("apm-server.server", monitoring.PublishExpvar)
	counter       = func(s string) *monitoring.Int {
		return monitoring.NewInt(serverMetrics, s)
	}
	requestCounter    = counter("request.count")
	concurrentWait    = counter("concurrent.wait.ms")
	responseCounter   = counter("response.count")
	responseErrors    = counter("response.errors.count")
	responseSuccesses = counter("response.valid.count")

	okResponse = serverResponse{
		nil, http.StatusOK, counter("response.valid.ok"),
	}
	acceptedResponse = serverResponse{
		nil, http.StatusAccepted, counter("response.valid.accepted"),
	}
	forbiddenCounter  = counter("response.errors.forbidden")
	forbiddenResponse = func(err error) serverResponse {
		return serverResponse{
			errors.Wrap(err, "forbidden request"), http.StatusForbidden, forbiddenCounter,
		}
	}
	unauthorizedResponse = serverResponse{
		errors.New("invalid token"), http.StatusUnauthorized, counter("response.errors.unauthorized"),
	}
	requestTooLargeResponse = serverResponse{
		errors.New("request body too large"), http.StatusRequestEntityTooLarge, counter("response.errors.toolarge"),
	}
	decodeCounter        = counter("response.errors.decode")
	cannotDecodeResponse = func(err error) serverResponse {
		return serverResponse{
			errors.Wrap(err, "data decoding error"), http.StatusBadRequest, decodeCounter,
		}
	}
	validateCounter        = counter("response.errors.validate")
	cannotValidateResponse = func(err error) serverResponse {
		return serverResponse{
			errors.Wrap(err, "data validation error"), http.StatusBadRequest, validateCounter,
		}
	}
	rateLimitedResponse = serverResponse{
		errors.New("too many requests"), http.StatusTooManyRequests, counter("response.errors.ratelimit"),
	}
	methodNotAllowedResponse = serverResponse{
		errors.New("only POST requests are supported"), http.StatusMethodNotAllowed, counter("response.errors.method"),
	}
	tooManyConcurrentRequestsResponse = serverResponse{
		errors.New("timeout waiting to be processed"), http.StatusServiceUnavailable, counter("response.errors.concurrency"),
	}
	fullQueueCounter  = counter("response.errors.queue")
	fullQueueResponse = func(err error) serverResponse {
		return serverResponse{
			errors.New("queue is full"), http.StatusServiceUnavailable, fullQueueCounter,
		}
	}
	serverShuttingDownCounter  = counter("response.errors.closed")
	serverShuttingDownResponse = func(err error) serverResponse {
		return serverResponse{
			errors.New("server is shutting down"), http.StatusServiceUnavailable, serverShuttingDownCounter,
		}
	}
)

func newMuxer(beaterConfig *Config, report reporter) *http.ServeMux {
	mux := http.NewServeMux()
	logger := logp.NewLogger("handler")
	for url, v1Route := range V1Routes {
		logger.Infof("Path %s added to request handler", url)
		mux.Handle(url, v1Route.Handler(beaterConfig, report))
	}

	for url, v2Route := range V2Routes {
		logger.Infof("Path %s added to request handler", url)
		mux.Handle(url, v2Route.Handler(beaterConfig, report))
	}

	mux.Handle(HealthCheckURL, healthCheckHandler())

	if beaterConfig.Expvar.isEnabled() {
		path := beaterConfig.Expvar.Url
		logger.Infof("Path %s added to request handler", path)
		mux.Handle(path, expvar.Handler())
	}
	return mux
}

func sendStatus(w http.ResponseWriter, r *http.Request, res serverResponse) {
	contentType := "text/plain; charset=utf-8"
	if acceptsJSON(r) {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(res.code)

	responseCounter.Inc()
	res.counter.Inc()
	if res.err == nil {
		log.Println("Error is nil")
		responseSuccesses.Inc()
		return
	}

	logger, ok := r.Context().Value(reqLoggerContextKey).(*logp.Logger)
	if !ok {
		logger = logp.NewLogger("request")
	}
	errMsg := res.err.Error()
	logger.Errorw("error handling request", "response_code", res.code, "error", errMsg)

	responseErrors.Inc()

	if acceptsJSON(r) {
		sendJSON(w, map[string]interface{}{"error": errMsg})
	} else {
		sendPlain(w, errMsg)
	}
}

func acceptsJSON(r *http.Request) bool {
	h := r.Header.Get("Accept")
	return strings.Contains(h, "*/*") || strings.Contains(h, "application/json")
}

func sendJSON(w http.ResponseWriter, msg map[string]interface{}) {
	buf, err := json.Marshal(msg)
	if err != nil {
		logp.NewLogger("response").Errorf("Error while generating a JSON error response: %v", err)
		return
	}

	w.Write(buf)
}

func sendPlain(w http.ResponseWriter, msg string) {
	w.Write([]byte(msg))
}
