// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func TestCRIBufferPreservesPartialEntries(t *testing.T) {
	cfg := NewConfig()
	cfg.Format = containerdFormat
	cfg.AddMetadataFromFilePath = false
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	require.NoError(t, err)
	var bodies []string
	out := helper.NewSynchronousLogEmitter(set, func(_ context.Context, es []*entry.Entry) {
		for _, e := range es {
			bodies = append(bodies, e.Body.(string))
		}
	})
	op.SetOutputIDs([]string{out.ID()})
	require.NoError(t, op.SetOutputs([]operator.Operator{out}))
	// The default internal emitter buffers output until Stop; partial entries also
	// survive across ProcessBatch calls inside the recombine operator.
	for _, line := range []string{"P first", "F second", "F third"} {
		e := entry.New()
		e.Body = "2024-04-13T07:59:37.505201169Z stdout " + line
		require.NoError(t, op.ProcessBatch(t.Context(), []*entry.Entry{e}))
	}
	require.NoError(t, op.Stop())
	require.Equal(t, []string{"firstsecond", "third"}, bodies)
}

func BenchmarkContainerBatchBuffer(b *testing.B) {
	for _, n := range []int{1, 100, 1000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			cfg := NewConfig()
			cfg.Format = containerdFormat
			cfg.AddMetadataFromFilePath = false
			op, err := cfg.Build(componenttest.NewNopTelemetrySettings())
			require.NoError(b, err)
			b.Cleanup(func() { require.NoError(b, op.Stop()) })
			es := make([]*entry.Entry, n)
			for i := range es {
				es[i] = entry.New()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				for _, e := range es {
					e.Body = "2024-04-13T07:59:37.505201169Z stdout F hello"
				}
				if err := op.ProcessBatch(b.Context(), es); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
