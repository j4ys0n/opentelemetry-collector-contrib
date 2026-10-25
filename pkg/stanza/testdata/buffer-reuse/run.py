# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

import json,time,subprocess,signal,sys,threading,collections,shutil
from pathlib import Path
import yaml,urllib.request
binary,tag,mode=sys.argv[1:4]
if not tag or Path(tag).name != tag or tag in ('.','..'):raise ValueError('tag must be a simple directory name')
n=int(sys.argv[4]) if len(sys.argv)>4 else 100000
root=Path('/tmp/stanza-e2e/runs')/tag
if root.exists():shutil.rmtree(root)
(root/'input').mkdir(parents=True)
expected={}; lines=[[] for _ in range(8)]
ts='2024-04-13T07:59:37.505201169Z'
for i in range(n):
 body=f'id={i:08d} '+('abcdefghijklmnopqrstuvwxyz0123456789'*12)
 expected[body]=1
 if mode=='docker':rows=[json.dumps({'log':body,'stream':'stdout','time':ts})+'\n']
 elif i%4==0:rows=[f'{ts} stdout P {body[:150]}\n',f'{ts} stdout F {body[150:]}\n']
 else:rows=[f'{ts} stdout F {body}\n']
 lines[i%8].extend(rows)
paths=[root/'input'/f'{i}.log' for i in range(8)]
for p,ls in zip(paths,lines):p.write_text('' if mode in ('live','rotate') else ''.join(ls))
cfg={'receivers':{'filelog':{'include':[str(root/'input'/'*.log')],'start_at':'beginning','include_file_path':True,'poll_interval':'20ms','operators':[{'type':'container','add_metadata_from_filepath':False}]}},'processors':{'batch':{'send_batch_size':1024,'timeout':'100ms'}},'exporters':{'file':{'path':str(root/'out.json'),'flush_interval':'100ms'}},'service':{'telemetry':{'logs':{'level':'error'},'metrics':{'level':'detailed','readers':[{'pull':{'exporter':{'prometheus':{'host':'127.0.0.1','port':18888}}}}]}},'pipelines':{'logs':{'receivers':['filelog'],'processors':['batch'],'exporters':['file']}}}}
(root/'config.yaml').write_text(yaml.safe_dump(cfg))
log=open(root/'stderr.txt','w');start=time.monotonic();p=subprocess.Popen([binary,'--config',str(root/'config.yaml')]+(['--feature-gates=stanza.synchronousLogEmitter'] if 'sync' in tag else []),stderr=log,stdout=log)
seen=collections.Counter()
def writer():
 time.sleep(.3)
 for offset in range(0,max(map(len,lines)),73):
  if mode=='rotate' and offset==146:
   time.sleep(.2)
   for path in paths:path.rename(path.with_suffix('.old'));path.touch()
  for path,ls in zip(paths,lines):
   with path.open('a') as f:f.writelines(ls[offset:offset+73])
  time.sleep(.005)
thread=None
if mode in ('live','rotate'):thread=threading.Thread(target=writer);thread.start()
f=None
try:
 while time.monotonic()-start<90:
  if p.poll() is not None:raise RuntimeError('collector exited '+(root/'stderr.txt').read_text())
  if f is None and (root/'out.json').exists():f=open(root/'out.json')
  if f:
   while True:
    pos=f.tell();line=f.readline()
    if not line or not line.endswith('\n'):f.seek(pos);break
    obj=json.loads(line)
    for rl in obj.get('resourceLogs',[]):
     for sl in rl.get('scopeLogs',[]):
      for rec in sl.get('logRecords',[]):seen[rec['body']['stringValue']]+=1
  if sum(seen.values())>=n:break
  time.sleep(.01)
 elapsed=time.monotonic()-start
 (root/'metrics.txt').write_bytes(urllib.request.urlopen('http://127.0.0.1:18888/metrics',timeout=5).read())
finally:
 if thread:thread.join()
 p.send_signal(signal.SIGTERM);p.wait(timeout=20);log.close()
 if f:f.close()
# Re-read after graceful shutdown to catch late duplicates or incomplete flushes.
seen=collections.Counter()
if (root/'out.json').exists():
 for line in (root/'out.json').open():
  for rl in json.loads(line).get('resourceLogs',[]):
   for sl in rl.get('scopeLogs',[]):
    for r in sl.get('logRecords',[]):seen[r['body']['stringValue']]+=1
result={'tag':tag,'mode':mode,'records':n,'seconds':round(elapsed,3),'correct':seen==expected,'missing':sum((collections.Counter(expected)-seen).values()),'extra':sum((seen-collections.Counter(expected)).values()),'exit':p.returncode,'stderr':(root/'stderr.txt').read_text()[:1000]}
for line in (root/'metrics.txt').read_text().splitlines():
 if line.startswith('otelcol_process_runtime_total_alloc_bytes '):result['allocated_mib']=round(float(line.split()[-1])/1048576,3)
 if line.startswith('otelcol_process_runtime_heap_alloc_bytes '):result['heap_mib']=round(float(line.split()[-1])/1048576,3)
print(json.dumps(result),flush=True)
with open('/tmp/stanza-e2e/results.jsonl','a') as f:f.write(json.dumps(result)+'\n')
if not result['correct'] or p.returncode:sys.exit(1)
