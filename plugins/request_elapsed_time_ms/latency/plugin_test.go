package latency_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mebenhoehta/temp-sloth-plugin/plugins/request_elapsed_time_ms/latency"
	"github.com/stretchr/testify/assert"
)

func TestGetBucketValues(t *testing.T) {
	tests := map[string]struct {
		latency, expLowBound, expUpperBound int
		expErr                              bool
	}{
		"below zero should fail": {
			latency: -1,
			expErr:  true,
		},
		"below lower bound": {
			latency:       1,
			expLowBound:   latency.LowestBucket,
			expUpperBound: latency.LowestBucket,
		},
		"exact lower bound ": {
			latency:       latency.LowestBucket,
			expLowBound:   latency.LowestBucket,
			expUpperBound: latency.LowestBucket,
		},
		"exact bound": {
			latency:       100,
			expLowBound:   100,
			expUpperBound: 100,
		},
		"exact upper bound": {
			latency:       latency.TopBucket,
			expLowBound:   latency.TopBucket,
			expUpperBound: latency.TopBucket,
		},
		"above max bound": {
			latency:       latency.TopBucket + 10000,
			expLowBound:   latency.TopBucket,
			expUpperBound: latency.TopBucket,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			lowerResult, upperResult, err := latency.GetBucketValues(test.latency)

			if test.expErr {
				asserts.Error(err)
			} else if asserts.NoError(err) {
				asserts.Equal(test.expLowBound, lowerResult)
				asserts.Equal(test.expUpperBound, upperResult)
			}
		})
	}
}

func TestGetBucketRatio(t *testing.T) {
	tests := map[string]struct {
		latency  int
		expRatio float32
	}{
		"exact bound latency": {
			latency:  250,
			expRatio: 0,
		},
		"mid bound latency": {
			latency:  175,
			expRatio: 0.5,
		},
		"almost bound latency": {
			latency:  101,
			expRatio: 0.006666666667,
		},
		"two thirds bound latency": {
			latency:  200,
			expRatio: 0.666666666667,
		},
		"exact lower bound latency": {
			latency:  250,
			expRatio: 0,
		},
		"below lowest bound latency": {
			latency:  1,
			expRatio: 0,
		},
		"exact upper bound latency": {
			latency:  latency.LowestBucket,
			expRatio: 0,
		},
		"above upper bound latency": {
			latency:  latency.TopBucket + 10000,
			expRatio: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			ratioResult := latency.GetBucketRatio(test.latency)

			asserts.Equal(test.expRatio, ratioResult)
		})
	}
}

func TestSLIPlugin(t *testing.T) {
	tests := map[string]struct {
		meta     map[string]string
		labels   map[string]string
		options  map[string]string
		expQuery string
		expErr   bool
	}{
		"Missing servicename, should fail.": {
			options: map[string]string{},
			expErr:  true,
		},

		"Missing latency option should fail.": {
			options: map[string]string{
				"servicename": "test",
			},
			expErr: true,
		},

		"Invalid latency should fail.": {
			options: map[string]string{
				"servicename": "test",
			},
			expErr: true,
		},

		"0 (or below) latency should fail.": {
			options: map[string]string{
				"servicename": "test",
				"latency":     "0",
			},
			expErr: true,
		},

		"Massive latency should fail.": {
			options: map[string]string{
				"servicename": "test",
				"latency":     "5000000",
			},
			expErr: true,
		},

		"Validation should run and fail on invalid data.": {
			options: map[string]string{"apm_tx_regex": "([xyz", "good_http_status_regex": "([xyz",
				"bad_http_status_regex": "([xyz"},
			expErr: true,
		},

		"A servicename and latency without filters should return a valid query.": {
			options: map[string]string{"servicename": "demandproduct", "latency": "100"},
			expQuery: `
1 - ((
	sum(rate(request:ELAPSED_TIME_MS_bucket{job="demandproduct-metrics", le="100.0"}[{{.window}}]))
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{job="demandproduct-metrics"}[{{.window}}])) > 0)
) OR on() vector(1))`,
		},

		"Typical options with latency on bucket definition should return a valid query.": {
			options: map[string]string{"servicename": "demandproduct", "apm_tx": "/product/full", "latency": "100"},
			expQuery: `
1 - ((
	sum(rate(request:ELAPSED_TIME_MS_bucket{job="demandproduct-metrics", APM_TRANSACTION="/product/full", le="100.0"}[{{.window}}]))
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{job="demandproduct-metrics", APM_TRANSACTION="/product/full"}[{{.window}}])) > 0)
) OR on() vector(1))`,
		},

		"Typical options should return a valid query.": {
			options: map[string]string{"servicename": "demandproduct", "apm_tx": "/product/full", "latency": "150"},
			expQuery: `
1 - ((
	(
	(1-0.333333) * sum(rate(request:ELAPSED_TIME_MS_bucket{job="demandproduct-metrics", APM_TRANSACTION="/product/full", le="100.0"}[{{.window}}]))
	+ 0.333333 * sum(rate(request:ELAPSED_TIME_MS_bucket{job="demandproduct-metrics", APM_TRANSACTION="/product/full", le="250.0"}[{{.window}}]))
	)
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{job="demandproduct-metrics", APM_TRANSACTION="/product/full"}[{{.window}}])) > 0)
) OR on() vector(1))`,
		},

		"Having all options set should return a valid query.": {
			options: map[string]string{
				"servicename":            "demandproduct",
				"latency":                "175",
				"apm_tx":                 "someapm",
				"apm_tx_regex":           ".*apm.*",
				"filter":                 `r="v",s="w"`,
				"success_filter":         `o="g",p="h"`,
				"good_http_status_regex": `[2-4]..`,
				"bad_http_status_regex":  `[404|302]`,
			},
			expQuery: `
1 - ((
	(
	(1-0.500000) * sum(rate(request:ELAPSED_TIME_MS_bucket{job="demandproduct-metrics", APM_TRANSACTION="someapm", APM_TRANSACTION=~".*apm.*", r="v", s="w", le="100.0", RESPONSE_STATUS=~"[2-4]..", RESPONSE_STATUS!~"[404|302]", o="g", p="h"}[{{.window}}]))
	+ 0.500000 * sum(rate(request:ELAPSED_TIME_MS_bucket{job="demandproduct-metrics", APM_TRANSACTION="someapm", APM_TRANSACTION=~".*apm.*", r="v", s="w", le="250.0", RESPONSE_STATUS=~"[2-4]..", RESPONSE_STATUS!~"[404|302]", o="g", p="h"}[{{.window}}]))
	)
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{job="demandproduct-metrics", APM_TRANSACTION="someapm", APM_TRANSACTION=~".*apm.*", r="v", s="w"}[{{.window}}])) > 0)
) OR on() vector(1))`,
		},
		"Filters should be sanitized with ','.": {
			options: map[string]string{
				"servicename":    "test",
				"latency":        "100",
				"success_filter": `,k1="v2",k2="v2",`,
			},
			expQuery: `
1 - ((
	sum(rate(request:ELAPSED_TIME_MS_bucket{job="test-metrics", k1="v2", k2="v2", le="100.0"}[{{.window}}]))
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{job="test-metrics"}[{{.window}}])) > 0)
) OR on() vector(1))`,
		},

		"Filter should be sanitized with '{'.": {
			options: map[string]string{
				"servicename":    "test",
				"latency":        "100",
				"success_filter": `{k1 = "v2", k2 = "v2"}, `,
			},
			expQuery: `
1 - ((
	sum(rate(request:ELAPSED_TIME_MS_bucket{job="test-metrics", k1="v2", k2="v2", le="100.0"}[{{.window}}]))
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{job="test-metrics"}[{{.window}}])) > 0)
) OR on() vector(1))`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			gotQuery, err := latency.SLIPlugin(context.TODO(), test.meta, test.labels, test.options)

			if test.expErr {
				asserts.Error(err)
			} else if asserts.NoError(err) {
				asserts.Equal(strings.Trim(test.expQuery, " \n\t"), strings.Trim(gotQuery, " \n\t"))
			}
		})
	}

}
