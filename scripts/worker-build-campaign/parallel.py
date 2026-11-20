#!/usr/bin/env python3
"""Replay remaining pinned cases on isolated workers, without duplicate jobs."""
import argparse, collections, concurrent.futures, datetime, json, os, pathlib, queue, subprocess, threading, time

REPO=pathlib.Path(__file__).resolve().parents[2]
OUT=REPO/'docs/worker-build-fixes-2026-11-16'

def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()

def atomic(path,data):
    temp=path.with_name(path.name+'.'+str(os.getpid())+'.'+str(threading.get_ident())+'.tmp')
    temp.write_text(json.dumps(data,indent=2));temp.replace(path)

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--baseline',type=pathlib.Path,required=True);parser.add_argument('--workers',type=int,default=4);args=parser.parse_args()
    if not 1<=args.workers<=4:raise RuntimeError('approved maximum is four concurrent cases')
    capacity=json.loads(subprocess.check_output(['docker','info','--format','{"cpus":{{.NCPU}},"memory_bytes":{{.MemTotal}}}'],text=True))
    # Keep the reference per-builder memory limit. Reserve memory for existing
    # services/host rather than overcommitting this VM's physical capacity.
    workers=min(args.workers,4,int((capacity['memory_bytes']-1536*1024**2)//(2*1024**3)))
    if workers<1:raise RuntimeError('insufficient VM memory for a reference-sized builder')
    for lane in range(2,workers+1):
        metadata=json.loads((OUT/'lanes'/('lane'+str(lane)+'.json')).read_text())
        if metadata['initial_images']:raise RuntimeError('new lane was not provisioned with empty storage')
    tasks=[]
    for case in json.loads((args.baseline/'manifest.json').read_text()):
        original=json.loads((args.baseline/'results'/(case['name']+'.json')).read_text())
        if not original.get('reference_qualified'):continue
        path=OUT/'results'/(case['name']+'.json')
        if path.exists():
            if json.loads(path.read_text()).get('complete'):continue
            raise RuntimeError('incomplete case requires inspection before dispatch: '+case['name'])
        # Shared reference Rust caches made later samples artificially short;
        # prioritize these cold worker builds to reduce the final long tail.
        estimate=600 if case['kind']=='rust' else original.get('reference',{}).get('seconds',0)
        tasks.append((estimate,case['name']))
    tasks.sort(key=lambda item:-item[0])
    pending=queue.Queue()
    for _,name in tasks:pending.put(name)
    lock=threading.Lock();stop=threading.Event()
    state={'started_at':now(),'finished_at':None,'workers':workers,'approved_maximum':{'workers':4,'builder_cpus':6,'builder_memory_bytes':8*1024**3},
           'actual_builder_limits':{'total_cpus':workers*1.5,'total_memory_bytes':workers*2*1024**3,'per_worker_cpus':1.5,'per_worker_memory_bytes':2*1024**3},
           'docker_capacity':capacity,'reserved_host_service_memory_bytes':1536*1024**2,
           'in_flight':{},'completed_parallel':[],'errors':[],
           'supporting_services':'shared run-owned test DB and broker; separate AMQP vhost, private network, mock endpoint, worker and build store per lane',
           'scheduling':'longest reference builds first; Rust estimated cold because reference cache was shared'}
    def save():atomic(OUT/'parallel-state.json',state)
    def update_progress():
        records=[json.loads(p.read_text()) for p in (OUT/'results').glob('*.json') if not p.name.endswith('.worker-image.json')]
        finished=[r for r in records if r.get('complete')]
        atomic(OUT/'progress.json',{'completed':len(finished),'denominator':57,'image_builds':sum(r['worker'].get('image_built',False) for r in finished),
               'runtime_passes':sum(bool(r['worker'].get('runtime',{}).get('ready')) for r in finished),
               'outcomes':dict(collections.Counter(r['classification'] for r in finished)),'parallel_workers':workers,'in_flight':dict(state['in_flight'])})
    def lane_loop(lane):
        while not stop.is_set():
            try:name=pending.get_nowait()
            except queue.Empty:return
            with lock:
                state['in_flight'][str(lane)]=name;save();print('START lane='+str(lane),name,flush=True)
            env=dict(os.environ);env['ZDFIX_LANE']=str(lane)
            log=OUT/('lane'+str(lane)+'-controller.log')
            with log.open('a') as stream:
                result=subprocess.run(['python3',str(REPO/'scripts/worker-build-campaign/rerun.py'),'--baseline',str(args.baseline),'--cases',name],env=env,stdout=stream,stderr=subprocess.STDOUT)
            with lock:
                state['in_flight'].pop(str(lane),None)
                if result.returncode:
                    state['errors'].append({'lane':lane,'case':name,'exit_code':result.returncode});stop.set();save()
                    print('ERROR lane='+str(lane),name,result.returncode,flush=True);return
                state['completed_parallel'].append({'lane':lane,'case':name,'finished_at':now()});save();update_progress()
                print('DONE lane='+str(lane),name,flush=True)
    save()
    with concurrent.futures.ThreadPoolExecutor(max_workers=workers) as pool:
        futures=[pool.submit(lane_loop,lane) for lane in range(1,workers+1)]
        for future in futures:future.result()
    state['finished_at']=now();save();update_progress()
    if state['errors']:raise RuntimeError('parallel replay stopped; inspect preserved case and controller logs')
    print('parallel replay complete',flush=True)

if __name__=='__main__':main()
