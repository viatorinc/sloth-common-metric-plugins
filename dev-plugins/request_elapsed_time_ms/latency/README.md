# Viator latency SLO plugin using  request:ELAPSED_TIME_MS

Latency plugin for services

SLIPluginID = "viator-sloth-plugins/request_elapsed_time_ms/availability"

Evaluates availability by using the ratio of the count of requests that are below equal a chosen latency  vs the tottal request count

When the chosen latency is between bucket boundaries, the ration between the two boundaries ise used,
assuming linear request distribution between boundaries. The error is increasing the bigger boundaries.

## Options

- `servicename`: Used to filter Prometheus jobs by appending `-metrics`
                 e.g. `payoutservice` used as `payoutservice-metrics` or `demandproduct` as `demandproduct-metrics`
- `latency`: the latency that is considered a "successful/good" response, anything above this is considered "bad"
- `apm_tx`: (**Optional**)  the APM_TRANSACTION to look at
- `apm_tx_regex`: (**Optional**) the APM_TRANSACTION to look at as a regex
- `filter`: (**Optional**) A general prometheus filter string using concatenated labels, used for total and success queries
                      defaults to unset
- `success_filter`: (**Optional**) A general prometheus filter string using concatenated labels, used for success queries
                      defaults to unset
- `good_http_status_regex`:  (**Optional**) a regex for HTTP status codes that are considered successful/good responses,
                      while this defaults to unset
- `bad_http_status_regex`:  (**Optional**) a regex of HTTP status codes that are bad responses
                      defaults to unset

See viator-sloth-plugins/plugins/request_elapsed_time_ms/availability/README.md for general filter options

Successful response are evaluated by filtering on:
* good/successful http statuses (`good_http_status_regex`)
* the success filter (`success_filter`)
* excluding bad/failed http status (`bad_http_status_regex`).

The response HTTP status are used to determine good(successful) and bad (failed) responses.
All the apm_tx and status options can be set in parallel. ie for status this would be valid:

    good_http_status_regex="[234]00"
    bad_http_status_regex="(423|405|307|202|200)"

and it would result in a promql of:

    RESPONSE_STATUS="[234]00", RESPONSE_STATUS!~"(423|400|405|307)"

And this result in only 200 and 300 being considered good/successful response.

Practically, it makes sense to stick with one of the options, though readability of the config should trump any other considerations.

Missing scrape values (ie server does not respond on `/metric` endpoint) are considered an error (ie 0.0 success rate)

If neither any of the status options nor the `success_filter` options are set, then `good_http_status_regex` defaults to "2.."
( pass a filter with " " to prevent that and create an SRE ticket for your use case)

There should not be many situation where an SLO is not limited to a single APM_TRANSACTION,
otherwise slow throughput endpoints could be drowned out by `/ping` calls.
Even combining unrelated calls can lead to dilution of the results.

## Metric requirements

- `request:ELAPSED_TIME_MS_count`: From experience-common.

https://gitlab.dev.tripadvisor.com/viator/engineering/experiences-common/-/blob/develop/experiences-common-shared/src/main/java/com/tripadvisor/experiences/common/shared/integration/monitoring/MonitoringData.java#L103-107

## Usage examples

### Without filter (minimum)

only response codes of 200-299 are considered successful responses

```yaml
sli:
  plugin:
    id: "viator-sloth-plugins/request_elapsed_time_ms/availability"
    options:
      servicename: "demandproduct"
      apm_tx: "/product/filter"
```

### With filters

```yaml
sli:
  plugin:
    id: "viator-sloth-plugins/request_elapsed_time_ms/availability"
    options:
      servicename: "demandproduct"
      apm_tx: "/product/filter"
      filter: REQUEST_SIZE_BUCKET="FIFTY", CLIENT="TRIPADVISOR"
      status_regex: "(2..|404)"
```
