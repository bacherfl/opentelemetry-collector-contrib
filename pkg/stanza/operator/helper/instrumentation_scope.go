// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"go.opentelemetry.io/collector/component"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

type InstrumentationScope struct {
	Name    string
	Version string
}

func NewInstrumentationScope(set component.TelemetrySettings) InstrumentationScope {
	s := InstrumentationScope{}

	if serviceName, ok := set.Resource.Attributes().Get(string(semconv.ServiceNameKey)); ok {
		s.Name = serviceName.AsString()
	}

	if version, ok := set.Resource.Attributes().Get(string(semconv.ServiceVersionKey)); ok {
		s.Version = version.AsString()
	}

	return s
}

func (i InstrumentationScope) AddInstrumentationScope(e *entry.Entry) {
	e.ScopeName = i.Name
	e.ScopeVersion = i.Version
}
