// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type retainingOutput struct {
	helper.OutputOperator
	retained []*entry.Entry
}

func (o *retainingOutput) ProcessBatch(_ context.Context, es []*entry.Entry) error {
	o.retained = es
	return nil
}

func TestProcessBatchRetainingDownstream(t *testing.T) {
	set := componenttest.NewNopTelemetrySettings()
	cfg := NewConfig()
	cfg.CombineField = entry.NewBodyField()
	cfg.IsLastEntry = "true"
	cfg.IfExpr = "false"
	op, err := cfg.Build(set)
	require.NoError(t, err)
	base, err := helper.NewOutputConfig("retaining", "retaining").Build(set)
	require.NoError(t, err)
	out := &retainingOutput{OutputOperator: base}
	op.SetOutputIDs([]string{out.ID()})
	require.NoError(t, op.SetOutputs([]operator.Operator{out}))
	e := entry.New()
	e.Body = "must survive downstream handoff"
	require.NoError(t, op.ProcessBatch(t.Context(), []*entry.Entry{e}))
	require.Same(t, e, out.retained[0], "downstream retained slice was corrupted")
}

func (*retainingOutput) Process(context.Context, *entry.Entry) error { return nil }
