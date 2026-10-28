// Regression coverage for bundled samples through the normal upload/Forge pipeline.
// Run with the shared Go browser-test server or pass --url explicitly.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import {createRequire} from "node:module";
import {fileURLToPath, pathToFileURL} from "node:url";
const root=path.resolve(path.dirname(fileURLToPath(import.meta.url)),"..");
const require=createRequire(import.meta.url),{chromium}=require(process.env.SCENE_PALETTE_PLAYWRIGHT||"playwright");
const option=(key,fallback)=>{const i=process.argv.indexOf(key);return i<0?fallback:process.argv[i+1];};
const url=new URL(option("--url","http://127.0.0.1:8765/"));
assert.ok(["127.0.0.1","localhost","[::1]"].includes(url.hostname));
const html=path.resolve(root,option("--html","index.html"));
const output=path.resolve(root,option("--output",`bin/e2e/samples-${Date.now()}`));
assert.ok(output.startsWith(path.join(root,"bin/e2e")+path.sep));await fs.mkdir(output,{recursive:true});
const report={startedAt:new Date().toISOString(),checks:[],errors:[],externalRequests:[],status:"running"};
const passed=name=>{report.checks.push(name);console.log("PASS "+name);};
const canonical=plate=>{const p=JSON.parse(JSON.stringify(plate));delete p.createdAt;delete p.review;return p;};
const snapshot=page=>page.evaluate(()=>JSON.parse(JSON.stringify(snapshot())));
async function settled(page){await page.waitForFunction(()=>state.core.length===8&&!document.getElementById("extractBtn").disabled);}
async function forge(page){await page.locator("#extractBtn").click();await settled(page);}
async function sampleSettled(page,id){await page.waitForFunction(id=>state.sampleId===id&&state.stills.length===1&&state.stills[0].name===`sample-${id}.jpg`&&state.core.length===8&&!document.getElementById("extractBtn").disabled,id);}
async function gateNextDecode(page){await page.evaluate(()=>{const original=window.createImageBitmap;let first=true;let release,finished;const pending=new Promise(r=>release=r),completed=new Promise(r=>finished=r);window.__sampleDecodeGate={release,original,started:false,completed};window.createImageBitmap=async(...args)=>{if(first){first=false;window.__sampleDecodeGate.started=true;await pending;try{return await original(...args);}finally{finished();}}return original(...args);};});}
async function releaseDecode(page){await page.evaluate(async()=>{const gate=window.__sampleDecodeGate;window.createImageBitmap=gate.original;gate.release();await gate.completed;await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));delete window.__sampleDecodeGate;});}
let browser;
try{
 browser=await chromium.launch({...(process.platform==="win32"?{channel:"msedge"}:{}),headless:true});
 const enginePalettes={};
 for(const [label,target]of [["Go",url.href],["standalone",pathToFileURL(html).href]]){
  const context=await browser.newContext({viewport:{width:1440,height:1000},acceptDownloads:true});
  if(label==="standalone")await context.setOffline(true);
  const page=await context.newPage();page.on("pageerror",e=>report.errors.push(label+": "+e.message));
  await context.route("**/*",route=>{const u=new URL(route.request().url());if(["http:","https:"].includes(u.protocol)&&!["127.0.0.1","localhost","[::1]"].includes(u.hostname)){report.externalRequests.push(u.href);return route.abort();}return route.continue();});
  await page.goto(target);await page.waitForFunction(()=>typeof state!=="undefined");
  assert.deepEqual(await page.evaluate(()=>({stills:state.stills.length,core:state.core.length})),{stills:0,core:0});
  assert.ok(await page.locator("#trySample").isVisible());
  assert.ok(await page.locator("#tryOn").isChecked());
  assert.equal(await page.locator('#samplePicker [data-sample="blue"]').count(),1);
  assert.equal(await page.locator("#samplePicker [data-sample]").count(),3);
  await page.screenshot({path:path.join(output,label+"-empty.png"),fullPage:true});
  await page.setViewportSize({width:390,height:1000});await page.screenshot({path:path.join(output,label+"-empty-mobile.png"),fullPage:true});await page.setViewportSize({width:1440,height:1000});
  passed(label+": empty workspace offers three samples without automatically loading/generating one");
  enginePalettes[label]={};
  for(const id of ["blue","violet","amber"]){
   await page.locator("#plateName").fill("Sample regression");
   await page.locator(id==="blue"?"#trySample":`#samplePicker [data-sample="${id}"]`).click();
   await page.waitForFunction(id=>state.sampleId===id&&state.stills.length===1&&state.core.length===8&&!document.getElementById("extractBtn").disabled,id);
   const sample=await snapshot(page);enginePalettes[label][id]=canonical(sample);
   assert.equal(await page.evaluate(()=>document.documentElement.style.getPropertyValue("--bg")),sample.core.find(t=>t.id==="ground").hex);
   assert.ok(await page.locator("#extractBtn").evaluate(e=>{const c=getComputedStyle(e);const rgb=s=>{const a=s.match(/[\d.]+/g).map(Number);return {r:a[0],g:a[1],b:a[2]}};return contrast(rgb(c.color),rgb(c.backgroundColor))>=4.5;}),"Primary controls stay readable in the generated theme");
   await page.locator("#tryOn").uncheck();assert.equal(await page.evaluate(()=>document.documentElement.style.getPropertyValue("--bg")),"");await page.locator("#tryOn").check();
   assert.equal(sample.core.length,8);assert.ok(await page.locator("#results").isVisible());
   const polarity=id==="blue"?"light":"dark";assert.equal(sample.polarity,polarity);assert.equal(await page.locator(`[data-pol="${polarity}"]`).getAttribute("aria-pressed"),"true");
   assert.equal(await page.locator(`#samplePicker [data-sample="${id}"]`).getAttribute("aria-pressed"),"true");
   await page.locator('[data-fmt="json"]').click();const download=page.waitForEvent("download");await page.locator("#downloadBtn").click();const saved=path.join(output,`${label}-${id}.json`);await(await download).saveAs(saved);
   assert.deepEqual(canonical(JSON.parse(await fs.readFile(saved))),canonical(sample));
   await page.screenshot({path:path.join(output,`${label}-${id}-generated.png`),fullPage:true});
   const original=await page.evaluate(()=>({name:state.stills[0].name,bytes:Array.from(new Uint8Array(state.stills[0].bytes))}));
   assert.equal(original.name,`sample-${id}.jpg`);
   await page.locator("#file").setInputFiles({name:original.name,mimeType:"image/jpeg",buffer:Buffer.from(original.bytes)});
   await page.waitForFunction(()=>state.sampleId===null&&state.stills.length===1);
   await forge(page);assert.deepEqual(canonical(await snapshot(page)),canonical(sample),label+" "+id+": sample and ordinary uploaded bytes must produce identical semantic roles, colours and provenance");

   passed(label+": "+id+" sample auto-generates exactly the uploaded-image palette and exports it");
  }
  const custom=path.join(root,"testdata/reference-images/pastel-prisms.png");
  await page.locator('#samplePicker [data-sample="blue"]').click();await sampleSettled(page,"blue");
  await page.locator("#file").setInputFiles(custom);await page.waitForFunction(()=>state.sampleId===null&&state.stills.length===1&&state.stills[0].name==="pastel-prisms.png");await forge(page);
  assert.equal((await snapshot(page)).sources[0].file,"pastel-prisms.png");
  passed(label+": custom upload replaces sample and follows normal Forge pipeline");
  const customPlate=canonical(await snapshot(page));await page.locator('#samplePicker [data-sample="violet"]').click();await sampleSettled(page,"violet");await page.locator("#toastAction").click();assert.equal(await page.evaluate(()=>state.sampleId),null);assert.deepEqual(canonical(await snapshot(page)),customPlate);passed(label+": sample Undo restores the complete preceding custom workspace");
  await gateNextDecode(page);await page.locator('#samplePicker [data-sample="blue"]').click();await page.waitForFunction(()=>window.__sampleDecodeGate.started);
  await page.locator('#samplePicker [data-sample="amber"]').click();await page.waitForFunction(()=>state.sampleId==="amber"&&state.core.length===8&&!document.getElementById("extractBtn").disabled);await releaseDecode(page);
  assert.equal(await page.evaluate(()=>state.sampleId),"amber");assert.equal(await page.evaluate(()=>state.stills[0].name),"sample-amber.jpg");
  passed(label+": a delayed earlier sample cannot overwrite a newer sample");
  await gateNextDecode(page);await page.locator('#samplePicker [data-sample="blue"]').click();await page.waitForFunction(()=>window.__sampleDecodeGate.started);
  await page.locator("#file").setInputFiles(custom);await page.waitForFunction(()=>state.stills.length===1&&state.stills[0].name==="pastel-prisms.png");await releaseDecode(page);await forge(page);
  assert.equal(await page.evaluate(()=>state.sampleId),null);assert.equal((await snapshot(page)).sources[0].file,"pastel-prisms.png");
  passed(label+": a custom upload supersedes a pending sample without stale replacement");
  await page.locator('#samplePicker [data-sample="blue"]').click();await sampleSettled(page,"blue");
  for(const width of [1440,390]){await page.setViewportSize({width,height:1000});const picker=await page.locator("#samplePicker").boundingBox();assert.ok(picker.x>=0&&picker.x+picker.width<=width);for(const button of await page.locator("#samplePicker [data-sample]").all())assert.ok(await button.isVisible());await page.screenshot({path:path.join(output,`${label}-blue-${width}.png`),fullPage:true});if(width===1440)await page.screenshot({path:path.join(output,label+"-preview.png"),fullPage:false});}
  await context.close();
 }
 for(const id of ["blue","violet","amber"])assert.deepEqual(enginePalettes.Go[id],enginePalettes.standalone[id],id+": exact Go/HTML sample parity");
 passed("All three sample palettes match exactly between Go and offline HTML");
 assert.deepEqual(report.errors,[]);assert.deepEqual(report.externalRequests,[]);passed("No unexpected browser errors or external requests");report.status="passed";
}catch(e){report.status="failed";report.failure=e.stack;console.error(e.stack);process.exitCode=1;}
finally{if(browser)await browser.close();report.finishedAt=new Date().toISOString();await fs.writeFile(path.join(output,"report.json"),JSON.stringify(report,null,2));console.log("Sample verification "+report.status+": "+path.join(output,"report.json"));}
