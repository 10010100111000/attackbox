import test from 'node:test'
import assert from 'node:assert/strict'
import {unpack,previewFiles,manifestHash,sha} from '../server/labs/archive.ts'
import {labZip} from './helpers/lab-zip.mjs'
test('Compose package previews nested file references and never interpolate host values',async()=>{
 const yaml="services:\n  web:\n    image: '${IMAGE}'\n    environment: {PASSWORD: '${PASSWORD}'}\n    volumes: ['./init.sql:/init.sql:ro']\n"
 const files=await unpack(labZip([{name:'lab/compose.yml',data:yaml},{name:'lab/init.sql',data:'select 1;'}]),'lab.zip')
 const p=previewFiles(files);assert.deepEqual(p.composeFiles,['lab/compose.yml']);assert.deepEqual(p.variables,['IMAGE','PASSWORD']);assert.deepEqual(p.missingFiles,[]);assert.deepEqual(p.issues,[]);assert.equal(p.services[0].image,'${IMAGE}')
 files.delete('lab/init.sql');assert.deepEqual(previewFiles(files).missingFiles,['lab/init.sql'])
 const a={'a.txt':sha('a'),'b.txt':sha('b')},b={'b.txt':sha('b'),'a.txt':sha('a')};assert.equal(manifestHash(a),manifestHash(b))
})
test('ZIP rejects traversal, symlinks, duplicate names and expansion beyond file limits',async()=>{
 for(const entries of [[{name:'../escape',data:'x'}],[{name:'link',data:'/etc/passwd',mode:0xa1ff}],[{name:'same',data:'a'},{name:'same',data:'b'}],[{name:'bomb',data:Buffer.alloc(4*1024*1024+1),compress:true}]])await assert.rejects(unpack(labZip(entries),'lab.zip'))
})
test('Compose preview surfaces unsafe or unavailable deployment features',async()=>{
 const p=previewFiles(await unpack(Buffer.from("services: {web: {image: 'test:local', build: {context: ., ssh: [default]}, ports: ['8080:80']}}"),'compose.yml'))
 assert.ok(p.issues.some(i=>i.includes('build.ssh')));assert.ok(p.notes.some(i=>i.includes('端口映射')));assert.deepEqual(p.services[0].ports,[{port:80,protocol:'tcp'}])
 await assert.rejects(unpack(Buffer.from('anything'),'bad.exe'))
})

test('Build preview recognizes nested contexts, Dockerfiles and writable package directories',()=>{
 const files=new Map([
 ['lab/compose.yml',Buffer.from("services: {web: {build: {context: ./app, dockerfile: Dockerfile, args: {VERSION: '${VERSION:-1}'}}, volumes: ['./config:/config:ro', './data:/data']}}")],
 ['lab/app/Dockerfile',Buffer.from('FROM scratch')],['lab/app/main.txt',Buffer.from('hello')],['lab/config/site.conf',Buffer.from('config')],['lab/data/seed.txt',Buffer.from('seed')]])
 const p=previewFiles(files);assert.deepEqual(p.issues,[]);assert.deepEqual(p.missingFiles,[]);assert.deepEqual(p.services[0].build,{context:'lab/app',dockerfile:'lab/app/Dockerfile'});assert.ok(p.notes.some(n=>n.includes('独立副本')));assert.ok(p.variables.includes('VERSION'))
 files.delete('lab/app/Dockerfile');assert.ok(previewFiles(files).missingFiles.includes('lab/app/Dockerfile'))
})
test('Build preview rejects remote contexts, escape and privileged build options',()=>{
 for(const build of ["'https://example.com/repo.git'","{context: ../}","{context: ., additional_contexts: {other: .}}","{context: ., secrets: [token]}"]){
  const p=previewFiles(new Map([['compose.yml',Buffer.from('services: {web: {build: '+build+'}}')],['Dockerfile',Buffer.from('FROM scratch')]]));assert.ok(p.issues.length)
 }
})
