package availability_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mebenhoehta/temp-sloth-plugin/plugins/request_elapsed_time_ms/availability"
	"github.com/stretchr/testify/assert"
)

func TestSLIPlugin(t *testing.T) {
	tests := map[string]struct {
		meta     map[string]string
		labels   map[string]string
		options  map[string]string
		expQuery string
		expErr   bool
	}{
		"A missing servicename option, should fail.": {
			options: map[string]string{},
			expErr:  true,
		},

		"Validation should run and fail on invalid data.": {
			options: map[string]string{"apm_tx_regex": "([xyz", "good_http_status_regex": "([xyz",
				"bad_http_status_regex": "([xyz"},
			expErr: true,
		},

		"A servicename without filters should return a valid query.": {
			options: map[string]string{"servicename": "demandproduct"},
			expQuery: `
1 - ((
	sum(rate(request:ELAPSED_TIME_MS_count{job="demandproduct-metrics", RESPONSE_STATUS=~"2.."}[{{.window}}]))
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{job="demandproduct-metrics"}[{{.window}}])) > 0)
) OR on() vector(1))
`,
		},
		"Having all options set should return a valid query.": {
			options: map[string]string{
				"servicename":            "demandproduct",
				"apm_tx":                 "someapm",
				"apm_tx_regex":           ".*apm.*",
				"filter":                 `r="v",s="w"`,
				"success_filter":         `o="g",p="h"`,
				"good_http_status_regex": `[2-4]..`,
				"bad_http_status_regex":  `[404|302]`,
			},
			expQuery: `
1 - ((
	sum(rate(request:ELAPSED_TIME_MS_count{job="demandproduct-metrics", APM_TRANSACTION="someapm", APM_TRANSACTION=~".*apm.*", r="v", s="w", RESPONSE_STATUS=~"[2-4]..", RESPONSE_STATUS!~"[404|302]", o="g", p="h"}[{{.window}}]))
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{job="demandproduct-metrics", APM_TRANSACTION="someapm", APM_TRANSACTION=~".*apm.*", r="v", s="w"}[{{.window}}])) > 0)
) OR on() vector(1))
`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			gotQuery, err := availability.SLIPlugin(context.TODO(), test.meta, test.labels, test.options)

			if test.expErr {
				asserts.Error(err)
			} else if asserts.NoError(err) {
				asserts.Equal(strings.Trim(test.expQuery, " \n\t"), strings.Trim(gotQuery, " \n\t"))
			}
		})
	}
}
