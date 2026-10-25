# Container buffer end-to-end comparison

Controlled test with generated data, not a production deployment. Real Collector
process: filelog -> container parser -> batch -> file exporter. Go 1.26, Linux AMD64.
Baseline: `87eb65f5e96d9d70315ee401515c330c105d2a63`. Candidate: same baseline with
only CRI array pooling. Generic recombine output array reuse has been removed.

## Results

28 successful runs; 4,400,000 output records checked for exact body equality,
missing records and duplicates. A separate output pass verified every timestamp,
stdout stream attribute, and source-file association. All checks passed.

- Twelve 100,000-record static runs, alternating baseline/candidate order.
- Six 500,000-record static runs, alternating order.
- Live appends and Docker JSON: both versions with default asynchronous emission
  and with the synchronous-emitter feature gate.
- Rename/create rotation during live writes: both versions.
- Eight files per run; 25% of CRI messages split into P/F fragments.
- Approximately 444-byte logical payloads. Graceful shutdown; no error logs.

500,000-record elapsed seconds: baseline [3.059, 2.990, 3.219],
candidate [3.022, 3.030, 3.119]. The median difference is about 1%.
Cumulative Go heap allocation medians: baseline 2969 MiB, candidate 2988 MiB.
Per-run ranges overlap. This does not establish an end-to-end speedup or memory
reduction; it did not reveal a reproducible performance regression either.
Do not present synthetic operator benchmark gains as whole-Collector gains.

Raw observations are in results.json. heap_mib is a final heap snapshot, not peak
or retained memory. allocated_mib is cumulative Go heap allocation. CPU/RSS were
discarded because the environment exposed inconsistent process IDs through /proc.
Wall time includes startup and flushing and ends after expected records are observed.
The checker shares the environment with the Collector; OS caches were not controlled.

Package tests pass with the race detector, including the retained-downstream-slice
regression test. End-to-end binaries were not race builds. These tests do not cover
remote exporter retries, disk-full behavior, abrupt crashes, arbitrary custom
operators, or long-running production loads. They cannot prove zero possible regressions.

## Reproduction

Requires Go 1.26, Python 3 and PyYAML. Use separate baseline/candidate checkouts.
Generate a manifest: `python build-config.py REPOSITORY OUTPUT_DIRECTORY > MANIFEST`.
From each checkout's internal/tools directory, use its pinned builder:
`go tool go.opentelemetry.io/collector/cmd/builder --config MANIFEST`.
Keep the resulting binaries separate.

Run `python run.py BINARY UNIQUE_TAG MODE RECORD_COUNT` sequentially for each binary.
Modes: static, live, docker, rotate. Include sync in the tag for synchronous emission.
Use 100000/500000 records for static runs, 20000 for the others.
The harness uses /tmp/stanza-e2e/runs/UNIQUE_TAG, replaces that tag's previous data,
and binds telemetry to local port 18888. It retains raw output, configuration,
stderr and telemetry; observations append to /tmp/stanza-e2e/results.jsonl.
Live writes append 73 lines per file every 5 ms. Rotation renames files after
146 lines and continues in new files at the original paths. Validation runs again
after shutdown to catch late duplicates.
