package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus counters and gauges for the engine.
type Metrics struct {
	WorkflowStarted   prometheus.Counter
	WorkflowCompleted prometheus.Counter
	WorkflowFailed    prometheus.Counter

	TasksDispatched prometheus.Counter
	TasksCompleted  prometheus.Counter
	TasksFailed     prometheus.Counter
	TaskRetries     prometheus.Counter
	TasksDead       prometheus.Counter
	TasksRunning    prometheus.Gauge

	QueueDepth prometheus.Gauge

	HTTPRequestDuration *prometheus.HistogramVec
	HTTPRequestsTotal   *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		WorkflowStarted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "workflow_engine_workflows_started_total",
			Help: "Total number of workflow executions started.",
		}),
		WorkflowCompleted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "workflow_engine_workflows_completed_total",
			Help: "Total number of workflow executions completed successfully.",
		}),
		WorkflowFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "workflow_engine_workflows_failed_total",
			Help: "Total number of workflow executions that failed.",
		}),

		TasksDispatched: promauto.NewCounter(prometheus.CounterOpts{
			Name: "workflow_engine_tasks_dispatched_total",
			Help: "Total number of tasks dispatched to the queue.",
		}),
		TasksCompleted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "workflow_engine_tasks_completed_total",
			Help: "Total number of tasks completed successfully.",
		}),
		TasksFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "workflow_engine_tasks_failed_total",
			Help: "Total number of tasks that failed.",
		}),
		TaskRetries: promauto.NewCounter(prometheus.CounterOpts{
			Name: "workflow_engine_task_retries_total",
			Help: "Total number of task retry attempts.",
		}),
		TasksDead: promauto.NewCounter(prometheus.CounterOpts{
			Name: "workflow_engine_tasks_dead_total",
			Help: "Total number of tasks moved to dead-letter queue.",
		}),
		TasksRunning: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "workflow_engine_tasks_running",
			Help: "Number of tasks currently being executed.",
		}),

		QueueDepth: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "workflow_engine_queue_depth",
			Help: "Approximate number of messages in the task queue.",
		}),

		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "workflow_engine_http_request_duration_seconds",
			Help:    "HTTP request latency.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),

		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "workflow_engine_http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),
	}
}
