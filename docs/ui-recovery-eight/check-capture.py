from pathlib import Path
import subprocess,os,json,time,hashlib
root=Path.cwd();e=root/'docs/ui-recovery-eight';log=e/'checks/codeql-capture-regression.txt'
frozen={p:hashlib.sha256(p.read_bytes()).hexdigest() for m in ['manifest-00-before.json','manifest-01-after.json','manifest-02-after.json'] for p in [e/m,*[e/'screenshots'/r['file'] for r in json.loads((e/m).read_text())['shots']]]}
manifest=e/'manifest-99-after.json';failure=e/'manifest-98-before.json'
assert not manifest.exists() and not failure.exists()
assert not list((e/'screenshots').glob('kyrecovery-99-after-*.png'))
with log.open('w') as out:
 def run(args,env=None):return subprocess.run(args,cwd=root,stdout=out,stderr=subprocess.STDOUT,env=env)
 out.write('Actual capture CLI regression: existing output, competing writer, build failure cleanup, successful 52-image capture.\n');out.flush()
 try:
  assert run(['node','docs/ui-recovery-eight/capture.mjs','02','after']).returncode!=0
  out.write('PASS: frozen stage refused before any capture writes.\n');out.flush()
  failed_env=os.environ.copy();failed_env['GOOS']='invalid-capture-test-os'
  assert run(['node','docs/ui-recovery-eight/capture.mjs','98','before'],failed_env).returncode!=0
  assert not failure.exists()
  out.write('PASS: build failure removed only its incomplete reservation.\n');out.flush()
  capture=subprocess.Popen(['node','docs/ui-recovery-eight/capture.mjs','99','after'],cwd=root,stdout=out,stderr=subprocess.STDOUT)
  for i in range(200):
   if manifest.exists():break
   assert capture.poll() is None
   time.sleep(.025)
  assert manifest.exists()
  assert run(['node','docs/ui-recovery-eight/capture.mjs','99','after']).returncode!=0
  out.write('PASS: competing writer refused the active reservation.\n');out.flush()
  assert capture.wait(timeout=180)==0
  m=json.loads(manifest.read_text());assert len(m['shots'])==52 and not m['uncommitted_source_sha256']
  for r in m['shots']:
   p=e/'screenshots'/r['file'];assert hashlib.sha256(p.read_bytes()).hexdigest()==r['sha256'] and not r['overflow']
  assert all(hashlib.sha256(p.read_bytes()).hexdigest()==pin for p,pin in frozen.items())
  out.write(f'PASS: 52 images and manifest created; {len(frozen)} frozen files unchanged.\n');out.flush()
 finally:
  if 'capture' in locals() and capture.poll() is None:capture.terminate();capture.wait(timeout=20)
  for p in (e/'screenshots').glob('kyrecovery-99-after-*.png'):p.unlink()
  manifest.unlink(missing_ok=True);failure.unlink(missing_ok=True)
print(log.read_text().splitlines()[-1])
