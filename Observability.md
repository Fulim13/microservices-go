# Observability

## Trace (OpenTelemetry - Jaeger)

- Collection of operations to handle a unique transaction
  ![alt text](<CleanShot 2026-09-17 at 5.40.08 PM.png>)

## Metrics (Prometheus)

1. SLA (Service Level Agreement)
   - Metrics Agreement between customers and service (99.9% availability, otherwise customer gets credit)
2. SLI (Serivce Level Indicator)
   - Show the service metric (99.95% of requests succeed)
3. SLO (Service Level Objective)
   - Metric that you aim for (99.9% requests should succeed)

- Usually, **percentiles are preferred over averages** because averages can be heavily influenced by outliers, which may hide the actual distribution of the data.
- For example, **P95 latency** means 95% of requests are completed within that latency, while the slowest 5% may take longer.

## Logs

#### Node-Level Loggging

- App → log file / stdout
- Log file can become too large
- Use log rotation to split logs into multiple files
- Logs still need to be collected and analyzed
  ![alt text](<CleanShot 2026-09-17 at 5.49.17 PM.png>)

#### Cluster-level Logging

- Each Kubernetes node has a logging agent (usually a DaemonSet)
- Agent collects logs from containers
- Agent sends logs to a central logging backend
- Backend can store, search, visualize, and alert on logs
  ![alt text](<CleanShot 2026-09-17 at 5.49.35 PM.png>)

ELK Stack: Elasticsearch-Logstash-Kibana: Elastic-
search is the logging backend, Logstash is some kind of log collector, and Kibana is the UI for logs

## Service Performance Monitoring

- App sends OpenTelemetry data → Jaeger
- Jaeger sends the data → Prometheus
- Prometheus stores it as time-series data
- Jaeger uses Prometheus to show latency graphs over time
- It can show percentile metrics like P50, P95, and P99
  ![alt text](<CleanShot 2026-09-17 at 5.57.45 PM.png>)
