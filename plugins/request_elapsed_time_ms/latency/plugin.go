package latency

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

var generalFilterTpl = template.Must(template.New("").Option("missingkey=error").Parse(
	`job="{{.servicename}}-metrics"
	{{- if .apm_tx -}}
	  , APM_TRANSACTION="{{.apm_tx}}"
	{{- end -}}
	{{- if .apm_tx_regex -}}
	  , APM_TRANSACTION=~"{{.apm_tx_regex}}"
	{{- end -}}
    {{- .filter -}}
`))

var successFilterTpl = template.Must(template.New("").Option("missingkey=error").Parse(
	`
    {{- if .good_http_status_regex -}}
      , RESPONSE_STATUS=~"{{.good_http_status_regex}}"
    {{- end -}}
    {{- if .bad_http_status_regex -}}
      , RESPONSE_STATUS!~"{{.bad_http_status_regex}}"
    {{- end -}}
    {{- .success_filter -}}
`))

func GetServiceName(options map[string]string) (string, error) {
	servicename := strings.TrimSpace(options["servicename"])

	if servicename == "" {
		return "", fmt.Errorf("servicename is mandatory")
	}

	return servicename, nil
}

func ValidateGeneralExpCommonFilterOptions(options map[string]string) error {
	errors := ""
	servicename := strings.TrimSpace(options["servicename"])
	if servicename == "" {
		errors += "servicename is mandatory."
	}

	for _, option := range []string{"apm_tx_regex", "good_http_status_regex", "bad_http_status_regex"} {
		value := options[option]
		if value != "" {
			_, err := regexp.Compile(value)
			if err != nil {
				errors += fmt.Sprintf("invalid regex '%v' for option '%s': %s.", value, option, err)
			}
		}
	}

	if errors != "" {
		return fmt.Errorf(errors)
	}
	return nil
}

func GetGeneralExpCommonFilter(options map[string]string) (string, error) {
	servicename, _ := GetServiceName(options)
	apmTx := options["apm_tx"]
	apmTxRegex := options["apm_tx_regex"]
	filter := options["filter"]

	var buf bytes.Buffer
	tplValues := map[string]string{
		"servicename":  servicename,
		"apm_tx":       apmTx,
		"apm_tx_regex": apmTxRegex,
		"filter":       PrepareFilter(filter),
	}

	err := generalFilterTpl.Execute(&buf, tplValues)
	if err != nil {
		return "", fmt.Errorf("could not render query template: %w", err)
	}

	return buf.String(), nil
}

// GetSuccessFilter returns a rendered template for successful requests.
// when `enforceSuccessFilter` is true, a default `goodHTTPStatusRegex = "2.."` will be returned if nothing else is set.
func GetSuccessFilter(options map[string]string, enforceSuccessFilter bool) (string, error) {
	successFilter := options["success_filter"]
	goodHTTPStatusRegex := options["good_http_status_regex"]
	badHTTPStatusRegex := options["bad_http_status_regex"]

	if enforceSuccessFilter && (successFilter == "" && goodHTTPStatusRegex == "" && badHTTPStatusRegex == "") {
		goodHTTPStatusRegex = "2.."
	}

	var buf bytes.Buffer
	tplValues := map[string]string{
		"success_filter":         PrepareFilter(successFilter),
		"good_http_status_regex": goodHTTPStatusRegex,
		"bad_http_status_regex":  badHTTPStatusRegex,
	}

	err := successFilterTpl.Execute(&buf, tplValues)
	if err != nil {
		return "", fmt.Errorf("could not render query template: %w", err)
	}

	return buf.String(), nil
}

var regxCommaFormat = regexp.MustCompile(", *")
var regxEquals = regexp.MustCompile(`\s*=\s*`)

func PrepareFilter(filter string) string {
	filter = strings.Trim(filter, "}{, ")
	if filter != "" {
		// make it prettier
		filter = ", " + filter
		filter = strings.Join(strings.Fields(filter), " ")
		filter = regxCommaFormat.ReplaceAllString(filter, ", ")
		filter = regxEquals.ReplaceAllString(filter, "=")
	}
	return filter
}

const (
	// SLIPluginVersion is the version of the plugin spec.
	SLIPluginVersion = "prometheus/v1"
	// SLIPluginID is the registering ID of the plugin.
	SLIPluginID = "viator-sloth-plugins/request_elapsed_time_ms/latency"
)

var queryForExactBucketsTpl = template.Must(template.New("").Parse(`
1 - ((
	sum(rate(request:ELAPSED_TIME_MS_bucket{ {{- .general_exp_common_filter -}} {{- .success_exp_common_filter -}}, le="{{ .le }}.0"}[{{"{{.window}}"}}]))
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{ {{- .general_exp_common_filter -}} }[{{"{{.window}}"}}])) > 0)
) OR on() vector(1))
`))

// When the latency is between two buckets, the good values are
// good = (lowerBucketValue + (highBucketValue-lowBucketValue) * ratio .
var queryForRatiosTpl = template.Must(template.New("").Parse(`
1 - ((
	(
	(1-{{.ratio}}) * sum(rate(request:ELAPSED_TIME_MS_bucket{ {{- .general_exp_common_filter -}}, le="{{ .lowerBucket }}.0" {{- .success_exp_common_filter -}}}[{{"{{.window}}"}}]))
	+ {{.ratio}} * sum(rate(request:ELAPSED_TIME_MS_bucket{ {{- .general_exp_common_filter -}}, le="{{ .upperBucket }}.0" {{- .success_exp_common_filter -}}}[{{"{{.window}}"}}]))
	)
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{ {{- .general_exp_common_filter -}} }[{{"{{.window}}"}}])) > 0)
) OR on() vector(1))
`))

// as defined here:
// https: //gitlab.dev.tripadvisor.com/viator/engineering/experiences-common/-/blob/develop/experiences-common-shared/src/main/java/com/tripadvisor/experiences/common/shared/performance/ResponseTimeBucket.java
var buckets = []int{5, 10, 25, 50, 75, 100, 250, 500, 1000, 2000, 3000, 5000, 10000, 20000, 60000, 120000, 500000}
var LowestBucket = buckets[0]
var TopBucket = buckets[len(buckets)-1]

// get histogram bucket values for a particular target latency.
func GetBucketValues(latency int) (lowerBound int, upperBound int, err error) {
	lowerBound = LowestBucket
	upperBound = TopBucket

	if latency < 0 {
		return 0, 0, fmt.Errorf("latency needs to be >= 0")
	}

	for _, bucket := range buckets {
		if bucket <= latency {
			lowerBound = bucket
		}
		if bucket >= latency {
			upperBound = bucket
			break
		}
	}
	return lowerBound, upperBound, nil
}

// get the ratio that the chosen latency results lies in between its bucket boundaries.
// if the latency is spot on a bucket, this is not being called and there is no need for math and this returns 0.
func GetBucketRatio(latency int) float32 {
	var lowerBound, upperBound int
	lowerBound, upperBound, _ = GetBucketValues(latency)

	if lowerBound-upperBound == 0 {
		return 0
	}

	var ratio = (float32(latency) - float32(lowerBound)) / (float32(upperBound) - float32(lowerBound))
	return float32(math.Round(float64(ratio*1e13))) / 1e13
}

// SLIPlugin will return a query that will return the availability error based on ELAPSED_TIME_MS_count service metrics.
func SLIPlugin(_ context.Context, _, _, options map[string]string) (string, error) {

	err := ValidateGeneralExpCommonFilterOptions(options)
	if err != nil {
		return "", err
	}
	serviceName, _ := GetServiceName(options)
	latency, err := validateLatencyOption(options)
	if err != nil {
		return "", err
	}

	generalFilter, err := GetGeneralExpCommonFilter(options)
	if err != nil {
		return "", fmt.Errorf("could not generate general filter for '%s': %w", serviceName, err)
	}

	generalSuccessFilter, err := GetSuccessFilter(options, false)
	if err != nil {
		return "", fmt.Errorf("could not generate general success filter for '%s': %w", serviceName, err)
	}

	lowerBucketValue, upperBucketValue, err := GetBucketValues(latency)
	if err != nil {
		// validation should prevent that
		return "", err
	}

	var b bytes.Buffer
	var data map[string]string
	var query *template.Template
	if lowerBucketValue == upperBucketValue {
		data = map[string]string{
			"general_exp_common_filter": generalFilter,
			"success_exp_common_filter": generalSuccessFilter,
			"le":                        strconv.Itoa(latency),
		}
		query = queryForExactBucketsTpl
	} else {
		latencyRatio := GetBucketRatio(latency)
		data = map[string]string{
			"general_exp_common_filter": generalFilter,
			"success_exp_common_filter": generalSuccessFilter,
			"lowerBucket":               strconv.Itoa(lowerBucketValue),
			"upperBucket":               strconv.Itoa(upperBucketValue),
			"ratio":                     strconv.FormatFloat(float64(latencyRatio), 'f', 6, 32),
		}
		query = queryForRatiosTpl
	}
	err = query.Execute(&b, data)
	if err != nil {
		return "", fmt.Errorf("could not render query template for '%s': %w", serviceName, err)
	}

	return b.String(), nil
}

func validateLatencyOption(options map[string]string) (int, error) {
	latencyString := strings.TrimSpace(options["latency"])

	if latencyString == "" {
		return 0, fmt.Errorf("latency is mandatory and needs to be a number less than %v", TopBucket)
	}

	latency, err := strconv.ParseInt(latencyString, 10, 32)
	if err != nil || int(latency) <= 0 || int(latency) > TopBucket {
		return 0, fmt.Errorf(
			"latency is mandatory and needs to be a number greater than 0 less than %v, but was '%v', %w",
			TopBucket, latencyString, err)
	}
	return int(latency), nil
}
