import test from 'node:test'
import assert from 'node:assert/strict'
import {createJiti} from 'jiti'
const jiti=createJiti(import.meta.url)
const {encryptContentToken,decryptContentToken}=await jiti.import('../server/content/credentials.ts')
const {parseRepositoryConfig,githubCommit}=await jiti.import('../server/content/github.ts')
test('repository credentials require a server key, use authenticated encryption and never disclose bad tokens',()=>{
 const old=process.env.CONTENT_CREDENTIALS_KEY
 try{
  delete process.env.CONTENT_CREDENTIALS_KEY
  assert.throws(()=>encryptContentToken('private-token'),/CONTENT_CREDENTIALS_KEY/)
  process.env.CONTENT_CREDENTIALS_KEY='ab'.repeat(32)
  const cipher=encryptContentToken('private-token')
  assert.ok(!cipher.includes('private-token'));assert.equal(decryptContentToken(cipher),'private-token')
  const parts=cipher.split('.');parts[1]=Buffer.alloc(16).toString('base64url')
  assert.throws(()=>decryptContentToken(parts.join('.')),/无法解密/)
  process.env.CONTENT_CREDENTIALS_KEY='cd'.repeat(32)
  assert.throws(()=>decryptContentToken(cipher),/无法解密/)
 }finally{if(old===undefined)delete process.env.CONTENT_CREDENTIALS_KEY;else process.env.CONTENT_CREDENTIALS_KEY=old}
})
test('GitHub config supports a selected directory without accepting arbitrary hosts or paths',()=>{
 assert.deepEqual(parseRepositoryConfig({repository:'https://github.com/owner/repo.git',directory:'tutorials/'}),{repository:'owner/repo',branch:'main',directory:'tutorials'})
 for(const repository of ['https://evil.example/repo','http://github.com/owner/repo','owner/repo?token=secret','../repo'])assert.throws(()=>parseRepositoryConfig({repository}))
 for(const directory of ['/etc','../private','a/../b','a\\b'])assert.throws(()=>parseRepositoryConfig({repository:'owner/repo',directory}))
})
test('GitHub requests fix host, carry a server-only Bearer header and redact upstream errors',async()=>{
 const previous=globalThis.$fetch
 try{
  globalThis.$fetch=async(url,options)=>{assert.ok(url.startsWith('https://api.github.com/repos/owner/repo/'));assert.equal(options.redirect,'error');assert.equal(options.headers.Authorization,'Bearer PRIVATE_TOKEN');return {sha:'a'.repeat(40)}}
  assert.equal(await githubCommit({repository:'owner/repo',branch:'main',directory:''},'PRIVATE_TOKEN'),'a'.repeat(40))
  globalThis.$fetch=async()=>{throw Object.assign(Error('PRIVATE_TOKEN'),{response:{status:403}})}
  await assert.rejects(githubCommit({repository:'owner/repo',branch:'main',directory:''},'PRIVATE_TOKEN'),error=>!error.message.includes('PRIVATE_TOKEN')&&error.statusCode===502)
 }finally{globalThis.$fetch=previous}
})
