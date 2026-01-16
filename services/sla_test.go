package services

import (
	"testing"
	"time"
)

func TestSlaConditionMatches(t *testing.T) {
	tests := []struct {
		name           string
		metric         SLAPolicyMetric
		expectedLabel  string
		expectedColor  string
		expectedType   string
		shouldMatch    bool
	}{
		{
			name: "Reply time - 2.5 hours remaining",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(2*time.Hour + 30*time.Minute),
				Stage:    "active",
				Metric:   "reply_time",
			},
			expectedLabel: "Less than 3 hours remaining",
			expectedColor: "#3498DB",
			expectedType:  MetricTypeReply,
			shouldMatch:   true,
		},
		{
			name: "Resolution time - 1.5 hours remaining",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(1*time.Hour + 30*time.Minute),
				Stage:    "active",
				Metric:   "resolution_time",
			},
			expectedLabel: "Less than 2 hours remaining",
			expectedColor: "#F1C40F",
			expectedType:  MetricTypeResolution,
			shouldMatch:   true,
		},
		{
			name: "Reply time - 45 minutes remaining",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(45 * time.Minute),
				Stage:    "active",
				Metric:   "first_reply_time",
			},
			expectedLabel: "Less than 1 hour remaining",
			expectedColor: "#FFA500",
			expectedType:  MetricTypeReply,
			shouldMatch:   true,
		},
		{
			name: "Resolution time - 20 minutes remaining",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(20 * time.Minute),
				Stage:    "active",
				Metric:   "full_resolution_time",
			},
			expectedLabel: "Less than 30 minutes remaining",
			expectedColor: "#FF8C00",
			expectedType:  MetricTypeResolution,
			shouldMatch:   true,
		},
		{
			name: "Reply time - 10 minutes remaining",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(10 * time.Minute),
				Stage:    "active",
				Metric:   "reply_time",
			},
			expectedLabel: "Less than 15 minutes remaining",
			expectedColor: "#FF0000",
			expectedType:  MetricTypeReply,
			shouldMatch:   true,
		},
		{
			name: "Resolution time - breached",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(-5 * time.Minute),
				Stage:    "active",
				Metric:   "resolution_time",
			},
			expectedLabel: "BREACHED",
			expectedColor: "#FF0000",
			expectedType:  MetricTypeResolution,
			shouldMatch:   true,
		},
		{
			name: "Inactive metric",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(1 * time.Hour),
				Stage:    "inactive",
				Metric:   "reply_time",
			},
			shouldMatch: false,
		},
		{
			name: "Unknown metric type",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(1 * time.Hour),
				Stage:    "active",
				Metric:   "unknown_metric",
			},
			shouldMatch: false,
		},
		{
			name: "More than 3 hours remaining - should not match",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(4 * time.Hour),
				Stage:    "active",
				Metric:   "reply_time",
			},
			shouldMatch: false,
		},
		{
			name: "SLA breached more than 3 hours ago - should not match",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(-4 * time.Hour), // Breached 4 hours ago
				Stage:    "active",
				Metric:   "reply_time",
			},
			shouldMatch: false,
		},
		{
			name: "SLA breached exactly at 3 hour threshold - should not match",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(-3*time.Hour - 1*time.Second), // Just over 3 hours ago
				Stage:    "active",
				Metric:   "reply_time",
			},
			shouldMatch: false,
		},
		{
			name: "SLA breached just under 3 hours ago - should match",
			metric: SLAPolicyMetric{
				BreachAt: time.Now().Add(-2*time.Hour - 59*time.Minute), // Just under 3 hours ago
				Stage:    "active",
				Metric:   "resolution_time",
			},
			expectedLabel: "BREACHED",
			expectedColor: "#FF0000",
			expectedType:  MetricTypeResolution,
			shouldMatch:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, color, metricType, matches := SlaConditionMatches(tt.metric)

			if matches != tt.shouldMatch {
				t.Errorf("Expected match=%v, got match=%v", tt.shouldMatch, matches)
			}

			if matches {
				if label != tt.expectedLabel {
					t.Errorf("Expected label=%s, got label=%s", tt.expectedLabel, label)
				}
				if color != tt.expectedColor {
					t.Errorf("Expected color=%s, got color=%s", tt.expectedColor, color)
				}
				if metricType != tt.expectedType {
					t.Errorf("Expected type=%s, got type=%s", tt.expectedType, metricType)
				}
			}
		})
	}
}
