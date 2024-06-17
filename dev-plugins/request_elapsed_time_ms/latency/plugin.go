package latency

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"text/template"

	"github.com/viatorinc/sloth-common-metric-plugins/dev-plugins/request_elapsed_time_ms/filters"
)

// see notes in project readme how and why the below marker is used
// no code must be above this marker
// FILTER-TEMPLATE-LOCATION

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

// as defined here (internal):
// experiences-common/-/blob/develop/experiences-common-shared/src/main/java/com/tripadvisor/experiences/common/shared/performance/ResponseTimeBucket.java.
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

	err := filters.ValidateGeneralExpCommonFilterOptions(options)
	if err != nil {
		return "", err
	}
	serviceName, _ := filters.GetServiceName(options)
	latency, err := validateLatencyOption(options)
	if err != nil {
		return "", err
	}

	generalFilter, err := filters.GetGeneralExpCommonFilter(options)
	if err != nil {
		return "", fmt.Errorf("could not generate general filter for '%s': %w", serviceName, err)
	}

	generalSuccessFilter, err := filters.GetSuccessFilter(options, false)
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
