import test from 'node:test'
import assert from 'node:assert/strict'
import {parseTargetTemplate} from '../shared/target-templates.ts'
const good={name:'Web lab',description:'Training',image:'nginx:1.27',sourceUrl:'https://github.com/example/lab',ports:[{port:80,protocol:'tcp'}],accessInstructions:'Open /',enabled:true}
test('template validation accepts pinned references and protocol-specific ports, strips unsupported runtime fields',()=>{
 for(const image of ['nginx:1.27','ghcr.io/example/lab:Release_1','localhost:5000/lab:1','example/lab@sha256:'+'a'.repeat(64)])assert.equal(parseTargetTemplate({...good,image}).image,image)
 const result=parseTargetTemplate({...good,name:' Lab ',ports:[{port:53,protocol:'tcp'},{port:53,protocol:'udp'}],privileged:true,volumes:['/:/host']})
 assert.equal(result.name,'Lab');assert.equal(result.ports.length,2);assert.ok(!('privileged'in result));assert.ok(!('volumes'in result))
})
test('template validation rejects unsafe or ambiguous configuration',()=>{
 for(const image of ['nginx','https://example.com/lab:1','example/Lab:1','nginx:','nginx:1 --privileged','registry:70000/lab:1','repo@sha256:abc','user:password@example.com/lab:1'])assert.throws(()=>parseTargetTemplate({...good,image}),image)
 for(const sourceUrl of ['javascript:alert(1)','file:///tmp/test','https://user:password@example.com'])assert.throws(()=>parseTargetTemplate({...good,sourceUrl}))
 for(const patch of [{name:' '},{enabled:'true'},{ports:[{port:0,protocol:'tcp'}]},{ports:[{port:65536,protocol:'tcp'}]},{ports:[{port:'80',protocol:'tcp'}]},{ports:[{port:80,protocol:'http'}]},{ports:[{port:80,protocol:'tcp'},{port:80,protocol:'tcp'}]},{ports:Array(21).fill({port:80,protocol:'tcp'})}])assert.throws(()=>parseTargetTemplate({...good,...patch}))
})
