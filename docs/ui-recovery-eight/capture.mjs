// Real, disposable unconfigured administrator instance. No recovery key/share generation.
import { createHash, randomBytes } from 'node:crypto';
import { execFileSync, spawn } from 'node:child_process';
import { createRequire } from 'node:module';
import { createServer } from 'node:net';
import { once } from 'node:events';
import { mkdtemp, mkdir, readFile, writeFile, rm, open } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir, type, release } from 'node:os';
import { fileURLToPath } from 'node:url';

const [stage, phase] = process.argv.slice(2);
if (!/^\d\d$/.test(stage ?? '') || !['before','after'].includes(phase)) throw new Error('usage: capture.mjs <nn> before|after');
const root = fileURLToPath(new URL('../../', import.meta.url));
const here = fileURLToPath(new URL('.', import.meta.url));
const manifestPath = join(here, `manifest-${stage}-${phase}.json`);
// Reserve the stage before any screenshot writes; another run cannot share its outputs.
let manifestFile;
try { manifestFile = await open(manifestPath, 'wx'); }
catch (e) { if (e.code === 'EEXIST') throw new Error('Refusing to replace a frozen or reserved manifest', { cause: e }); throw e; }
let dir, server, browser, exited, captured = false;
try {
const require = createRequire(join(root,'tests/browser/package.json'));
const { chromium } = require('@playwright/test');
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const git = (...args) => execFileSync('git',['-C',root,...args]);
dir = await mkdtemp(join(tmpdir(),'kyrecovery-council-'));
const binary = join(dir,'kyrecovery');
execFileSync('go',['build','-o',binary,'./cmd/kyrecovery'],{cwd:root,stdio:'inherit'});
const listener = createServer();listener.listen(0,'127.0.0.1');await once(listener,'listening');
const port=listener.address().port;await new Promise(resolve=>listener.close(resolve));
const base=`http://127.0.0.1:${port}`, password=randomBytes(24).toString('base64url');
const fixtureFile=join(dir,'fixture.json');
await writeFile(fixtureFile,JSON.stringify({base,password,root,stage,phase}),{mode:0o600,flag:'wx'});
server=spawn(binary,['serve','--port',String(port),'--data-dir',join(dir,'data')],{
 cwd:dir,stdio:['ignore','ignore','inherit'],env:{PATH:process.env.PATH,HOME:dir,KY_ADMIN_INITIAL_PASSWORD:password,KYRECOVERY_COOKIE_SECURE:'false'},
});
exited=once(server,'exit');
 for(let i=0;;i++){
  if(server.exitCode!==null)throw new Error(`Server exited: ${server.exitCode}`);
  try {const res=await fetch(base+'/healthz',{signal:AbortSignal.timeout(1000)});if(res.ok)break;}catch{}
  if(i>=100)throw new Error('Server did not become ready');await new Promise(r=>setTimeout(r,100));
 }
 browser=await chromium.launch();
 const setup=await browser.newContext({baseURL:base});
 const login=await setup.request.post('/api/auth/login/local',{data:{username:'admin',password}});
 if(!login.ok())throw new Error(`Login failed: ${login.status()}`);
 const storageState=await setup.storageState();await setup.close();
 const surfaces=[
  ...[['overview','Overview','admin-console'],['backups','Backups','admin-console'],['capsules','Capsules','admin-console'],['pairing','Paired Products','admin-console'],['custodians','Custodians & Quorum','settings'],['replication','Offsite Replication','settings'],['audit','Audit Ledger','admin-console'],['appearance','Appearance','settings']].map(([surface,tab,cohort])=>({surface,tab,cohort})),
  {surface:'pairing-form',tab:'Paired Products',cohort:'admin-console',button:'New pairing code',modal:'#pairing-gen-modal'},
  {surface:'custodian-form',tab:'Custodians & Quorum',cohort:'settings',button:'+ Register Custodian',modal:'#custodian-modal'},
  {surface:'replication-form',tab:'Offsite Replication',cohort:'settings',button:'Add target',modal:'#replication-modal'},
  {surface:'ceremony',cohort:'settings',path:'/admin/ceremony'},
  {surface:'login',cohort:'auth',anonymous:true},
 ];
 const shots=[];const out=join(here,'screenshots');await mkdir(out,{recursive:true});
 for(const theme of ['light','dark'])for(const [device,viewport] of [['desktop',{width:1280,height:900}],['mobile',{width:390,height:844}]])for(const s of surfaces){
  const ctx=await browser.newContext({baseURL:base,colorScheme:theme,viewport,deviceScaleFactor:1,...(!s.anonymous&&{storageState})});
  const page=await ctx.newPage();await page.goto(s.path??'/');
  if(s.anonymous)await page.locator('#login-gateway-view').waitFor();
  else if(s.path)await page.locator('#ceremony-form').waitFor();
  else {
   await page.locator('#dashboard-view').waitFor();await page.locator('nav.nav-tabs').getByRole('button',{name:s.tab,exact:true}).click();
   await page.waitForFunction(()=>!document.querySelector('.tab-content.active')?.textContent.includes('Loading backup status'));
   if(s.button){await page.getByRole('button',{name:s.button,exact:true}).click();await page.locator(s.modal).waitFor();}
  }
  await page.evaluate(()=>document.fonts.ready);await page.waitForTimeout(300);
  const file=`kyrecovery-${stage}-${phase}-${s.surface}-${device}-${theme}.png`;
  const bytes=await page.screenshot({path:join(out,file),fullPage:false});
  const geometry=await page.evaluate(()=>({overflow:document.documentElement.scrollWidth-document.documentElement.clientWidth,renderedViewport:[innerWidth,innerHeight],theme:document.documentElement.dataset.theme,savedChoice:localStorage.getItem('kyrecovery-theme')}));
  shots.push({file,sha256:sha(bytes),bytes:bytes.length,width:viewport.width,height:viewport.height,viewport,device,theme,cohort:s.cohort,surface:s.surface,state:s.button?'open form':'empty/unconfigured',...geometry});
  await ctx.close();
 }
 const sourceNames=['internal/server/static/index.html','internal/server/static/css/styles.css','internal/server/static/ceremony.html','internal/server/static/css/ceremony.css','internal/server/static/js/app.js','internal/server/static/js/theme.js','internal/server/static/js/ceremony.js','internal/server/static/ky-ui/VERSION'];
 const source_files=Object.fromEntries(await Promise.all(sourceNames.map(async file=>[file,sha(await readFile(join(root,file)))])));
 const diff=git('diff','HEAD','--binary','--','internal/server/static');
 const manifest={product:'KyRecovery',stage,phase,councilRound:Number(process.env.COUNCIL_ROUND??(Number(stage)+1)),source_commit:git('rev-parse','HEAD').toString().trim(),uncommitted_source_sha256:diff.length?sha(diff):null,source_files,captured_at:new Date().toISOString(),os:`${type()} ${release()}`,driver:`Playwright ${require('@playwright/test/package.json').version}; Chromium ${browser.version()}; DPR1; normal service-worker behavior`,fixture:'Disposable administrator, no ceremony/key/capsules/pairings/custodians/targets; open forms are not submitted. Initial viewport only.',served_index_sha256:sha(Buffer.from(await(await fetch(base+'/')).arrayBuffer())),shots};
 await manifestFile.writeFile(JSON.stringify(manifest,null,2)+'\n');
 captured = true;
 await browser.close();browser=null;
 console.log(JSON.stringify({manifest:manifestPath,shots:shots.length,overflows:shots.filter(s=>s.overflow),fixtureFile,base}));
 if(process.env.KEEP_FIXTURE==='1')await new Promise(resolve=>{process.once('SIGTERM',resolve);process.once('SIGINT',resolve);});
} finally {
 await browser?.close();
 if(server && server.exitCode===null && server.signalCode===null)server.kill('SIGTERM');
 if(exited)await exited;
 if(dir)await rm(dir,{recursive:true,force:true});
 await manifestFile.close();
 if(!captured)await rm(manifestPath,{force:true});
}
