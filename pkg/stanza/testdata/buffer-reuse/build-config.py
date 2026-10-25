# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0
"""Generate an OCB manifest: repository output_directory."""
import sys
from pathlib import Path
import yaml
repo, output = [Path(p).resolve() for p in sys.argv[1:3]]
v = 'v0.163.1-0.20261022201525-42ee5d39b18f'
s = 'v1.69.1-0.20261022201525-42ee5d39b18f'
c = 'github.com/open-telemetry/opentelemetry-collector-contrib/'
config = {'dist': {'name': 'stanza-e2e', 'module': 'example.com/stanza-e2e', 'output_path': str(output), 'version': '0.163.0-dev'},
'receivers': [{'gomod': c+'receiver/filelogreceiver v0.163.0'}],
'exporters': [{'gomod': c+'exporter/fileexporter v0.163.0'}],
'processors': [{'gomod': 'go.opentelemetry.io/collector/processor/batchprocessor '+v}],
'providers': [{'gomod': 'go.opentelemetry.io/collector/confmap/provider/'+p+' '+s} for p in ['fileprovider','envprovider']], 'replaces': []}
for p in repo.rglob('go.mod'):
 module=p.read_text().splitlines()[0].removeprefix('module ')
 if module.startswith(c.rstrip('/')):config['replaces'].append(module+' => '+str(p.parent))
print(yaml.safe_dump(config))
