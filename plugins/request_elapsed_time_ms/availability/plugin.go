package availability

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
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
	SLIPluginID = "viator-sloth-plugins/request_elapsed_time_ms/availability"
)

var queryTpl = template.Must(template.New("").Parse(`
1 - ((
	sum(rate(request:ELAPSED_TIME_MS_count{ {{- .general_exp_common_filter -}} {{- .success_exp_common_filter -}}  }[{{"{{.window}}"}}]))
	/
	(sum(rate(request:ELAPSED_TIME_MS_count{ {{- .general_exp_common_filter -}} }[{{"{{.window}}"}}])) > 0)
) OR on() vector(1))
`))

// SLIPlugin will return a query that will return the availability error based on ELAPSED_TIME_MS_count service metrics.
func SLIPlugin(_ context.Context, _, _, options map[string]string) (string, error) {

	err := ValidateGeneralExpCommonFilterOptions(options)
	if err != nil {
		return "", err
	}

	serviceName, _ := GetServiceName(options)

	generalFilter, err := GetGeneralExpCommonFilter(options)
	if err != nil {
		return "", fmt.Errorf("could not generate general filter for '%s': %w", serviceName, err)
	}

	generalSuccessFilter, err := GetSuccessFilter(options, true)
	if err != nil {
		return "", fmt.Errorf("could not generate general success filter for '%s': %w", serviceName, err)
	}

	// Create query.
	var b bytes.Buffer
	data := map[string]string{
		"general_exp_common_filter": generalFilter,
		"success_exp_common_filter": generalSuccessFilter,
	}
	err = queryTpl.Execute(&b, data)
	if err != nil {
		return "", fmt.Errorf("could not render query template for '%s': %w", serviceName, err)
	}

	return b.String(), nil
}
