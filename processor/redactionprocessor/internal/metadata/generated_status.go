// Code generated by mdatagen. DO NOT EDIT.

package metadata

import (
	"go.opentelemetry.io/collector/component"
)

var (
	Type      = component.MustNewType("redaction")
	ScopeName = "github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor"
)

const (
	TracesStability  = component.StabilityLevelBeta
	LogsStability    = component.StabilityLevelAlpha
	MetricsStability = component.StabilityLevelAlpha
)
