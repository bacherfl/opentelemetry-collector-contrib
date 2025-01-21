// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestInstrumentationScope_AddInstrumentationScope(t *testing.T) {
	type fields struct {
		Name    string
		Version string
	}
	type args struct {
		e *entry.Entry
	}
	tests := []struct {
		name               string
		fields             fields
		args               args
		expectScopeName    string
		expectScopeVersion string
	}{
		{
			name: "add scope name and version",
			fields: fields{
				Name:    "my-scope",
				Version: "v1.0.0",
			},
			args: args{
				e: entry.New(),
			},
			expectScopeName:    "my-scope",
			expectScopeVersion: "v1.0.0",
		},
		{
			name: "add scope name",
			fields: fields{
				Name: "my-scope",
			},
			args: args{
				e: entry.New(),
			},
			expectScopeName: "my-scope",
		},
		{
			name: "add scope version",
			fields: fields{
				Version: "v1.0.0",
			},
			args: args{
				e: entry.New(),
			},
			expectScopeVersion: "v1.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := component.TelemetrySettings{}
			set.Resource = pcommon.NewResource()
			set.Resource.Attributes().PutStr(string(semconv.ServiceNameKey), tt.fields.Name)
			set.Resource.Attributes().PutStr(string(semconv.ServiceVersionKey), tt.fields.Version)
			i := NewInstrumentationScope(set)
			i.AddInstrumentationScope(tt.args.e)

			assert.Equal(t, tt.expectScopeName, tt.args.e.ScopeName)
			assert.Equal(t, tt.expectScopeVersion, tt.args.e.ScopeVersion)
		})
	}
}
