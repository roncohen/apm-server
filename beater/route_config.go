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
	"net/http"
	"regexp"

	"github.com/elastic/apm-server/processor"
	perr "github.com/elastic/apm-server/processor/error"
	"github.com/elastic/apm-server/processor/metric"
	"github.com/elastic/apm-server/processor/sourcemap"
	"github.com/elastic/apm-server/processor/transaction"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/transform"
	"github.com/elastic/beats/libbeat/logp"
)

var (
	rootURL                   = "/"
	BackendTransactionsURL    = "/v1/transactions"
	ClientSideTransactionsURL = "/v1/client-side/transactions"
	RumTransactionsURL        = "/v1/rum/transactions"
	BackendErrorsURL          = "/v1/errors"
	ClientSideErrorsURL       = "/v1/client-side/errors"
	RumErrorsURL              = "/v1/rum/errors"
	MetricsURL                = "/v1/metrics"
	SourcemapsClientSideURL   = "/v1/client-side/sourcemaps"
	SourcemapsURL             = "/v1/rum/sourcemaps"
	V2BackendURL              = "/v2/intake"
	V2RumURL                  = "/v2/rum/intake"

	HealthCheckURL = "/healthcheck"
)

type routeType struct {
	wrappingHandler     func(*Config, http.Handler) http.Handler
	configurableDecoder func(*Config, decoder.ReqDecoder) decoder.ReqDecoder
	transformConfig     func(*Config) transform.Config
}

type v1Route struct {
	routeType
	processor.Processor
}

type v2Route struct {
	routeType
}

var V1Routes = map[string]v1Route{
	BackendTransactionsURL:    {backendRouteType, transaction.Processor},
	ClientSideTransactionsURL: {rumRouteType, transaction.Processor},
	RumTransactionsURL:        {rumRouteType, transaction.Processor},
	BackendErrorsURL:          {backendRouteType, perr.Processor},
	ClientSideErrorsURL:       {rumRouteType, perr.Processor},
	RumErrorsURL:              {rumRouteType, perr.Processor},
	MetricsURL:                {metricsRouteType, metric.Processor},
	SourcemapsClientSideURL:   {sourcemapRouteType, sourcemap.Processor},
	SourcemapsURL:             {sourcemapRouteType, sourcemap.Processor},
}

var V2Routes = map[string]v2Route{
	V2BackendURL: {backendRouteType},
	V2RumURL:     {rumRouteType},
}

var (
	backendRouteType = routeType{
		backendHandler,
		backendMetadataDecoder,
		func(*Config) transform.Config { return transform.Config{} },
	}
	rumRouteType = routeType{
		rumHandler,
		rumMetadataDecoder,
		rumTransformConfig,
	}
	metricsRouteType = routeType{
		metricsHandler,
		backendMetadataDecoder,
		func(*Config) transform.Config { return transform.Config{} },
	}
	sourcemapRouteType = routeType{
		sourcemapHandler,
		backendMetadataDecoder,
		rumTransformConfig,
	}
)

func backendHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		concurrencyLimitHandler(beaterConfig,
			authHandler(beaterConfig.SecretToken, h)))
}

func rumHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
		concurrencyLimitHandler(beaterConfig,
			ipRateLimitHandler(beaterConfig.RumConfig.RateLimit,
				corsHandler(beaterConfig.RumConfig.AllowOrigins, h))))
}

func metricsHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.Metrics.isEnabled(),
			authHandler(beaterConfig.SecretToken, h)))
}

func sourcemapHandler(beaterConfig *Config, h http.Handler) http.Handler {
	return logHandler(
		killSwitchHandler(beaterConfig.RumConfig.isEnabled(),
			authHandler(beaterConfig.SecretToken, h)))
}

func backendMetadataDecoder(beaterConfig *Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeSystemData(d, beaterConfig.AugmentEnabled)
}

func rumMetadataDecoder(beaterConfig *Config, d decoder.ReqDecoder) decoder.ReqDecoder {
	return decoder.DecodeUserData(d, beaterConfig.AugmentEnabled)
}

func rumTransformConfig(beaterConfig *Config) transform.Config {
	smapper, err := beaterConfig.RumConfig.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	config := transform.Config{
		SmapMapper:          smapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.RumConfig.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.RumConfig.ExcludeFromGrouping),
	}
	return config
}
