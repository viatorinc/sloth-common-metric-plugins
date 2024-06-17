package filters_test

import (
	"fmt"
	"testing"

	"github.com/mebenhoehta/temp-sloth-plugin/dev-plugins/request_elapsed_time_ms/filters"
	"github.com/stretchr/testify/assert"
)

func TestGetServiceName(t *testing.T) {
	tests := map[string]struct {
		options             map[string]string
		expectedServiceName string
		expErr              bool
	}{
		"Missing servicename, should fail.": {
			options: map[string]string{},
			expErr:  true,
		},
		"set servicename should pass and be trimmed.": {
			options: map[string]string{
				"servicename": " demandproduct ",
			},
			expectedServiceName: "demandproduct",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			serviceName, err := filters.GetServiceName(test.options)
			fmt.Println("error: ", err)

			if test.expErr {
				asserts.Error(err)
			} else if asserts.NoError(err) {
				asserts.Equal(test.expectedServiceName, serviceName)
			}
		})
	}
}

func TestValidateGeneralExpCommonFilterOptions(t *testing.T) {
	tests := map[string]struct {
		options             map[string]string
		expectedServiceName string
		expErr              bool
		expErrMessage       string
	}{
		"Missing servicename, should fail.": {
			options:       map[string]string{},
			expErr:        true,
			expErrMessage: "servicename is mandatory.",
		},
		"servicename should be trimmed.": {
			options: map[string]string{
				"servicename": " demandproduct ",
			},
			expectedServiceName: "demandproduct",
		},
		"invalid apm_tx_regex should fail.": {
			options: map[string]string{
				"servicename":  "demandproduct",
				"apm_tx_regex": "([xyz",
			},
			expErr:        true,
			expErrMessage: "invalid regex '([xyz' for option 'apm_tx_regex': error parsing regexp: missing closing ]: `[xyz`.",
		},
		"invalid good_http_status_regex should fail.": {
			options: map[string]string{
				"servicename":            "demandproduct",
				"good_http_status_regex": "([xyz",
			},
			expErr:        true,
			expErrMessage: "invalid regex '([xyz' for option 'good_http_status_regex': error parsing regexp: missing closing ]: `[xyz`.",
		},
		"invalid bad_http_status_regex should fail.": {
			options: map[string]string{
				"servicename":           "demandproduct",
				"bad_http_status_regex": "([xyz",
			},
			expErrMessage: "invalid regex '([xyz' for option 'bad_http_status_regex': error parsing regexp: missing closing ]: `[xyz`.",
			expErr:        true,
		},
		"valid options should pass": {
			options: map[string]string{
				"servicename":            "demandproduct",
				"apm_tx_regex":           "url.*",
				"good_http_status_regex": "[23]00",
				"bad_http_status_regex":  "5..",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			err := filters.ValidateGeneralExpCommonFilterOptions(test.options)

			if test.expErr {
				asserts.Error(err)
				asserts.Equal(test.expErrMessage, err.Error())
			} else {
				asserts.NoError(err)
			}
		})
	}
}

func TestPrepareFilter(t *testing.T) {
	tests := map[string]struct {
		inpoutFilter   string
		expectedFilter string
	}{
		"filter should be trimmed from extra chars and prepended with comma": {
			inpoutFilter:   "{ a=b },",
			expectedFilter: ", a=b",
		},
		"empty filter should be left empty": {
			inpoutFilter:   "",
			expectedFilter: "",
		},
		"exta white spae should be removed": {
			inpoutFilter:   "a       =        b,     c  = d  ",
			expectedFilter: ", a=b, c=d",
		},
		"filter commas should be followed by single space": {
			inpoutFilter:   "a=b,c=d,    e=f",
			expectedFilter: ", a=b, c=d, e=f",
		},
		"filter equals sign should not be sounded by whitespace": {
			inpoutFilter:   "a\n=\tb",
			expectedFilter: ", a=b",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			outputFilter := filters.PrepareFilter(test.inpoutFilter)

			asserts.Equal(test.expectedFilter, outputFilter)
		})
	}
}

func TestGetGeneralExpCommonFilter(t *testing.T) {
	tests := map[string]struct {
		options              map[string]string
		expectedPartialQuery string
	}{
		"Setting all options should pass": {
			options: map[string]string{
				"servicename":  "demandproduct",
				"apm_tx":       "someapm",
				"apm_tx_regex": ".*apm.*",
				"filter":       `r="v",s="w"`,
			},
			expectedPartialQuery: "job=\"demandproduct-metrics\", APM_TRANSACTION=\"someapm\", APM_TRANSACTION=~\".*apm.*\", r=\"v\", s=\"w\"",
		},
		"Setting only servicename should pass": {
			options: map[string]string{
				"servicename": "demandproduct",
			},
			expectedPartialQuery: "job=\"demandproduct-metrics\"",
		},
		"Setting only service and apm options should pass": {
			options: map[string]string{
				"servicename": "demandproduct",
				"apm_tx":      "someapm",
			},
			expectedPartialQuery: "job=\"demandproduct-metrics\", APM_TRANSACTION=\"someapm\"",
		},
		"Setting only service and apm regex options should pass": {
			options: map[string]string{
				"servicename":  "demandproduct",
				"apm_tx_regex": ".*apm.*",
			},
			expectedPartialQuery: "job=\"demandproduct-metrics\", APM_TRANSACTION=~\".*apm.*\"",
		},
		"Setting only service and filter should pass": {
			options: map[string]string{
				"servicename": "demandproduct",
				"filter":      `,{r="v",s="w"},`,
			},
			expectedPartialQuery: "job=\"demandproduct-metrics\", r=\"v\", s=\"w\"",
		},

		"Filters should be sanitized.": {
			options: map[string]string{
				"servicename": "test",
				"filter":      `  {, { k1="v2",k2="v2"}, `,
			},
			expectedPartialQuery: "job=\"test-metrics\", k1=\"v2\", k2=\"v2\"",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			expPartialQuery, err := filters.GetGeneralExpCommonFilter(test.options)

			asserts.NoError(err)
			asserts.Equal(test.expectedPartialQuery, expPartialQuery)
		})
	}
}

func TestGetSuccessFilter(t *testing.T) {
	tests := map[string]struct {
		options              map[string]string
		enforceSuccessFilter bool
		expectedPartialQuery string
	}{
		"Setting all options should pass": {
			options: map[string]string{
				"good_http_status_regex": "[24]..",
				"bad_http_status_regex":  "4..*",
				"success_filter":         `r="v",s="w"`,
			},
			expectedPartialQuery: ", RESPONSE_STATUS=~\"[24]..\", RESPONSE_STATUS!~\"4..*\", r=\"v\", s=\"w\"",
		},
		"Setting nothing should pass without enforceSuccessFilter being set": {
			options:              map[string]string{},
			enforceSuccessFilter: false,
			expectedPartialQuery: "",
		},
		"Setting nothing should pass with enforceSuccessFilter being set": {
			options:              map[string]string{},
			enforceSuccessFilter: true,
			expectedPartialQuery: ", RESPONSE_STATUS=~\"2..\"",
		},
		"Setting only the good status should pass": {
			options: map[string]string{
				"good_http_status_regex": "(2..|30.)",
			},
			expectedPartialQuery: ", RESPONSE_STATUS=~\"(2..|30.)\"",
		},
		"Setting only the bad status should pass": {
			options: map[string]string{
				"bad_http_status_regex": "5..",
			},
			expectedPartialQuery: ", RESPONSE_STATUS!~\"5..\"",
		},
		"Setting only service and success_filter should pass": {
			options: map[string]string{
				"success_filter": `,{r="v",s="w"},`,
			},
			expectedPartialQuery: ", r=\"v\", s=\"w\"",
		},

		"Filters should be sanitized.": {
			options: map[string]string{
				"success_filter": `  {, { k1="v2",k2="v2"}, `,
			},
			expectedPartialQuery: ", k1=\"v2\", k2=\"v2\"",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			asserts := assert.New(t)

			expPartialQuery, err := filters.GetSuccessFilter(test.options, test.enforceSuccessFilter)

			asserts.NoError(err)
			asserts.Equal(test.expectedPartialQuery, expPartialQuery)
		})
	}
}
