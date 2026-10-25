# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

import json,re
from pathlib import Path
count=0;runs=0
for d in Path('/tmp/stanza-e2e/runs').iterdir():
 if not re.fullmatch(r'(baseline|reuse)-(static-\d|large-\d|live(-sync)?|docker(-sync)?|rotate)',d.name):continue
 for line in (d/'out.json').open():
  for rl in json.loads(line).get('resourceLogs',[]):
   for sl in rl.get('scopeLogs',[]):
    for r in sl.get('logRecords',[]):
     assert r['timeUnixNano']=='1712995177505201169',(d,r)
     attrs={a['key']:a['value'].get('stringValue') for a in r['attributes']}
     i=int(r['body']['stringValue'][3:11]);assert attrs['log.iostream']=='stdout'
     assert Path(attrs['log.file.path']).name==f'{i%8}.log',(d,attrs,i)
     count+=1
 runs+=1
print({'runs':runs,'records_checked':count,'metadata_valid':True})
