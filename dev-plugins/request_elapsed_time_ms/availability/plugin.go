package availability

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/mebenhoehta/temp-sloth-plugin/dev-plugins/request_elapsed_time_ms/filters"
)

// see notes in project readme how and why the below marker is used
// no code must be above this marker
// FILTER-TEMPLATE-LOCATION

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

	err := filters.ValidateGeneralExpCommonFilterOptions(options)
	if err != nil {
		return "", err
	}

	serviceName, _ := filters.GetServiceName(options)

	generalFilter, err := filters.GetGeneralExpCommonFilter(options)
	if err != nil {
		return "", fmt.Errorf("could not generate general filter for '%s': %w", serviceName, err)
	}

	generalSuccessFilter, err := filters.GetSuccessFilter(options, true)
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
