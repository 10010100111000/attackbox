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
 const p=previewFiles(await unpack(Buffer.from("services: {web: {image: 'test:local', build: ., ports: ['8080:80']}}"),'compose.yml'))
 assert.ok(p.issues.some(i=>i.includes('build')));assert.ok(p.notes.some(i=>i.includes('端口映射')));assert.deepEqual(p.services[0].ports,[{port:80,protocol:'tcp'}])
 await assert.rejects(unpack(Buffer.from('anything'),'bad.exe'))
})
