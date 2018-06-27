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

package schema

const ModelSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metrics/metric.json",
    "type": "object",
    "description": "Metric data captured by an APM agent",
    "properties": {
        "samples": {
            "type": ["object"],
            "description": "Sampled application metrics collected from the agent",
            "regexProperties": true,
            "patternProperties": {
                "^[^*\"]*$": {
                        "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/metrics/sample.json",
    "type": ["object", "null"],
    "description": "A single metric sample.",
    "anyOf": [
        {
            "properties": {
                "type": {
                    "description": "Counters and gauges capture a single value at a point in time.  Counter are cumulative, strictly increasing or decreasing, and typically most useful with derivative aggregations.  Gauges increase and decrease over time.",
                    "enum": ["counter", "gauge"]
                },
                "unit": {
                    "type": ["string", "null"]
                },
                "value": {"type": "number"}
            },
            "required": ["type", "value"]
        },
        {
            "properties": {
                "type": {
                    "description": "Summary metrics capture client-side aggregations describing the distribution of a metric",
                    "enum": ["summary"]
                },
                "unit": {
                    "description": "The unit of measurement of this metric eg: bytes. Only informational at this time",
                    "type": ["string", "null"]
                },
                "count": {
                    "description": "The total count of all observations for this metric",
                    "type": "number"
                },
                "sum": {
                    "description": "The sum of all observations for this metric",
                    "type": "number"
                },
                "stddev": {
                    "description": "The standard deviation describing this metric",
                    "type": ["number", "null"]
                },
                "min": {
                    "description": "The minimum value observed for this metric",
                    "type": ["number", "null"]
                },
                "max": {
                    "description": "The maximum value observed for this metric",
                    "type": ["number", "null"]
                },
                "quantiles": {
                    "description": "A list of quantiles describing the metric",
                    "type": ["array", "null"],
                    "items": {
                        "descrption": "A [quantile, value] tuple",
                        "type": ["array", "null"],
                        "items": [
                            {
                                "type": "number",
                                "minimum": 0, "maximum": 1
                            },
                            {
                                "type": "number"
                            }
                        ],
                        "maxItems": 2,
                        "minItems": 2
                    }
                }
            },
            "required": ["type", "count", "sum"]
        }
    ]
                }
            },
            "additionalProperties": false
        },
        "tags": {
            "type": ["object", "null"],
            "description": "A flat mapping of user-defined tags with string values",
            "regexProperties": true,
            "patternProperties": {
                "^[^*\"]*$": {
                    "type": ["string", "null"],
                    "maxLength": 1024
                }
            },
            "additionalProperties": false
        },
        "timestamp": {
            "type": "string",
            "format": "date-time",
            "pattern": "Z$",
            "description": "Recorded time of the metric, UTC based and formatted as YYYY-MM-DDTHH:mm:ss.sssZ"
        }
    },
    "required": ["samples", "timestamp"]
}
`
