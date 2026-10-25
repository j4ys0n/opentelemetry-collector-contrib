// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

func BenchmarkProcessBatchBuffer(b *testing.B) {
	for _, n := range []int{1, 100, 1000} {
		for _, mode := range []string{"skip", "complete"} {
			b.Run(fmt.Sprintf("%s/%d", mode, n), func(b *testing.B) {
				cfg := NewConfig()
				cfg.CombineField = entry.NewBodyField()
				cfg.IsLastEntry = "true"
				if mode == "skip" {
					cfg.IfExpr = "false"
				}
				op, err := cfg.Build(componenttest.NewNopTelemetrySettings())
				if err != nil {
					b.Fatal(err)
				}
				count := 0
				out := helper.NewSynchronousLogEmitter(componenttest.NewNopTelemetrySettings(), func(_ context.Context, es []*entry.Entry) { count += len(es) })
				op.SetOutputIDs([]string{out.ID()})
				if err = op.SetOutputs([]operator.Operator{out}); err != nil {
					b.Fatal(err)
				}
				es := make([]*entry.Entry, n)
				for i := range es {
					es[i] = entry.New()
					es[i].Body = "hello"
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if err = op.ProcessBatch(b.Context(), es); err != nil {
						b.Fatal(err)
					}
				}
				b.StopTimer()
				if count != b.N*n {
					b.Fatal(count)
				}
			})
		}
	}
}

func TestOutputBufferPreservesBufferedEntries(t *testing.T) {
	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsLastEntry = "true"
	cfg.IfExpr = "false"
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	out := helper.NewBatchingLogEmitter(set, func(_ context.Context, es []*entry.Entry) {
		for _, e := range es {
			if e == nil {
				t.Fatal("cleared retained entry")
			}
			got = append(got, e.Body.(string))
		}
	}, helper.WithMaxBatchSize(100))
	op.SetOutputIDs([]string{out.ID()})
	if err = op.SetOutputs([]operator.Operator{out}); err != nil {
		t.Fatal(err)
	}
	for batch := range 2 {
		es := make([]*entry.Entry, 60)
		for i := range es {
			es[i] = entry.New()
			es[i].Body = fmt.Sprint(batch*60 + i)
		}
		if err = op.ProcessBatch(t.Context(), es); err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != 120 {
		t.Fatal(len(got))
	}
	for i, v := range got {
		if v != fmt.Sprint(i) {
			t.Fatalf("entry %d: %s", i, v)
		}
	}
}

func TestOutputBufferClearedAndBounded(t *testing.T) {
	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsLastEntry = "true"
	cfg.IfExpr = "false"
	op, err := cfg.Build(componenttest.NewNopTelemetrySettings())
	if err != nil {
		t.Fatal(err)
	}
	transformer := op.(*Transformer)
	e := entry.New()
	e.Body = "hello"
	for _, n := range []int{100, 1, 65537, 1} {
		es := make([]*entry.Entry, n)
		for i := range es {
			es[i] = e
		}
		if err := op.ProcessBatch(t.Context(), es); err != nil {
			t.Fatal(err)
		}
		if len(transformer.outputEntries) != 0 {
			t.Fatal("buffer length was not reset")
		}
		if cap(transformer.outputEntries) > 65536 {
			t.Fatal("oversized buffer retained")
		}
		for _, retained := range transformer.outputEntries[:cap(transformer.outputEntries)] {
			if retained != nil {
				t.Fatal("entry reference retained")
			}
		}
	}
}

func TestOutputBufferConcurrentBatches(t *testing.T) {
	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsLastEntry = "true"
	cfg.IfExpr = "false"
	set := componenttest.NewNopTelemetrySettings()
	op, err := cfg.Build(set)
	if err != nil {
		t.Fatal(err)
	}
	var count atomic.Int64
	out := helper.NewSynchronousLogEmitter(set, func(_ context.Context, es []*entry.Entry) { count.Add(int64(len(es))) })
	op.SetOutputIDs([]string{out.ID()})
	if err := op.SetOutputs([]operator.Operator{out}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				e := entry.New()
				e.Body = "hello"
				if err := op.ProcessBatch(t.Context(), []*entry.Entry{e}); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	if count.Load() != 800 {
		t.Fatal(count.Load())
	}
}
