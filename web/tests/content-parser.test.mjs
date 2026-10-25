import test from 'node:test'
import assert from 'node:assert/strict'
import {readFile,mkdtemp,writeFile,mkdir,symlink,rm} from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import {spawnSync} from 'node:child_process'
import {parseModuleMarkdown,learnerContentPreview,ContentFormatError} from '../server/content/parse-module.ts'
import {checkContentDirectory,resolveContentImage} from '../server/content/read-directory.ts'
import {renderMarkdown} from '../utils/markdown.ts'
const module = (body, head='') => `---\n${head}\n---\n\n${body}`
const quiz = (content='prompt: 哪个命令？\nanswer: secret-standard-answer') => '```quiz\n'+content+'\n```'
function invalid(source,pattern){assert.throws(()=>parseModuleMarkdown('示例.md',source),error=>error instanceof ContentFormatError&&pattern.test(error.message)&&error.issues.every(i=>i.file==='示例.md'&&i.line>0))}

test('single-file prototype parses without authored IDs and strips answers from learner data',async()=>{
 const source=await readFile(new URL('../../prototypes/git-content-demo/Linux 基础.md',import.meta.url),'utf8')
 const parsed=parseModuleMarkdown('Linux 基础.md',source)
 assert.equal(parsed.title,'Linux 基础');assert.equal(parsed.chapters.length,3)
 assert.deepEqual(parsed.chapters.map(c=>c.questions.length),[1,1,1])
 assert.equal(parsed.chapters[0].questions[0].answer,'whoami')
 assert.ok(!parsed.chapters[0].bodyMarkdown.includes('normalization:'))
 assert.equal(parsed.sourceHash,parseModuleMarkdown('Linux 基础.md',source).sourceHash)
})
test('defaults and per-chapter overrides align with existing access and duration rules',()=>{
 const parsed=parseModuleMarkdown('教程.md',module('# A\n\n正文\n\n# B\n\n正文', 'chapters:\n  A:\n    access: public\n    durationMinutes: 15'))
 assert.equal(parsed.categorySlug,null);assert.equal(parsed.difficulty,'beginner')
 assert.deepEqual(parsed.chapters.map(c=>[c.access,c.durationMinutes]),[['public',15],['authenticated',null]])
})
test('AST distinguishes real chapters from code, nested headings and quoted headings',()=>{
 const parsed=parseModuleMarkdown('教程.md',module('# **Shell**\n\n```bash\n# not a chapter\n```\n\n> # Quote\n\n## Section\n\n# Permissions\n\n正文'))
 assert.deepEqual(parsed.chapters.map(c=>c.title),['Shell','Permissions'])
 assert.match(parsed.chapters[0].bodyMarkdown,/# not a chapter/)
 assert.match(parsed.chapters[0].bodyMarkdown,/> # Quote/)
})
test('BOM, CRLF, setext headings and source locations are supported',()=>{
 const parsed=parseModuleMarkdown('示例.md','\uFEFF'+module('Shell\n=====\n\n'+quiz()).replaceAll('\n','\r\n'))
 assert.equal(parsed.chapters[0].title,'Shell');assert.equal(parsed.chapters[0].source.line,5)
 assert.equal(parsed.chapters[0].questions[0].source.line,8)
})
test('quiz output is allow-listed and ordinary HTML remains escaped by the existing renderer',()=>{
 const parsed=parseModuleMarkdown('示例.md',module('# A\n\n<script>alert(1)</script>\n\n'+quiz('prompt: 查看命令？\nanswer: secret-standard-answer\nerrorHint: secret-hint')))
 const preview=learnerContentPreview(parsed),json=JSON.stringify(preview)
 assert.ok(!json.includes('secret-standard-answer')&&!json.includes('secret-hint'))
 assert.ok(!json.includes('normalization')&&!json.includes('sourceHash')&&!json.includes('matchKey'))
 const html=renderMarkdown(preview.chapters[0].bodyMarkdown).html
 assert.ok(!html.includes('<script>'));assert.match(html,/&lt;script&gt;/)
})
test('identity is independent of order and answers; duplicate identities are rejected',()=>{
 const a=parseModuleMarkdown('示例.md',module('# A\n\n'+quiz()+'\n\n# B\n\n正文'))
 const b=parseModuleMarkdown('示例.md',module('# B\n\n正文\n\n# A\n\n'+quiz('prompt: 哪个命令？\nanswer: changed')))
 assert.equal(a.chapters[0].matchKey,b.chapters[1].matchKey)
 assert.equal(a.chapters[0].questions[0].matchKey,b.chapters[1].questions[0].matchKey)
 invalid(module('# A\n\n# A'),/标题重复/)
 invalid(module('# A\n\n'+quiz()+'\n\n'+quiz()),/题干重复/)
})
test('header and chapter errors report locations without guessing or dropping content',()=>{
 invalid('# A',/配置头/);invalid('---\nsummary: x',/结束/)
 invalid(module('正文\n# A'),/正文必须/);invalid(module('## A'),/一级标题/)
 invalid(module('# A','difficulty: expert'),/difficulty/)
 invalid(module('# A','access: private'),/access/)
 invalid(module('# A','id: forbidden'),/未知配置/)
 invalid(module('# A','chapters:\n  Missing:\n    access: public'),/对应/)
 invalid(module('# A','chapters:\n  A:\n    durationMinutes: 0'),/时长/)
 invalid(module('# A','category: [linux]'),/category/)
 invalid(module('# A','learningOutcomes: [true]'),/目标/)
 invalid(module('# A','summary: first\nsummary: second'),/重复字段/)
})
test('invalid or nested quizzes cannot accidentally render answer configuration',()=>{
 invalid(module('# A\n\n```quiz\nprompt: x\nanswer: secret'),/未闭合/)
 invalid(module('# A\n\n> ```quiz\n> prompt: x\n> answer: secret\n> ```'),/顶层/)
 invalid(module('# A\n\n    ```quiz\n    prompt: x\n    answer: secret\n    ```'),/缩进/)
 invalid(module('# A','description: |\n  ```quiz\n  prompt: x\n  answer: y\n  ```'),/模块介绍/)
 invalid(module('# A\n\n'+quiz().replace('```quiz','```QUIZ')),/独立/)
 invalid(module('# A\n\n'+quiz('prompt: x\nanswer: true')),/文本/)
 invalid(module('# A\n\n'+quiz('prompt: x\nanswer: "  "')),/不能为空/)
 invalid(module('# A\n\n'+quiz('prompt: x\nanswer: y\nnormalization: invalid')),/normalization/)
 const broken=module('# A\n\n'+quiz('prompt: x\nanswer: [PRIVATE_BAD_ANSWER'))
 try{parseModuleMarkdown('示例.md',broken);assert.fail()}catch(e){assert.ok(!e.message.includes('PRIVATE_BAD_ANSWER'));assert.equal(e.issues[0].line,10)}
 invalid(module('# A\n\n'+quiz('prompt: &anchor x\nanswer: *anchor')),/别名/)
})
test('repository validation discovers images, ignores README and does not read symlinks',async()=>{
 const root=await mkdtemp(path.join(os.tmpdir(),'content-parser-'))
 try{
  await mkdir(path.join(root,'images'));await writeFile(path.join(root,'images','sample.png'),Buffer.from([137,80,78,71]))
  await writeFile(path.join(root,'教程.md'),module('# A\n\n![示意](images/sample.png)'))
  await writeFile(path.join(root,'README.md'),'This is not a module')
  const [result]=await checkContentDirectory(root)
  assert.equal(result.images[0].repositoryPath,'images/sample.png')
  const relative=await resolveContentImage(root,{file:'nested/教程.md',line:8,href:'../images/sample.png'})
  assert.equal(relative.repositoryPath,'images/sample.png')
  for(const href of ['../outside.png','%2e%2e/outside.png','https://example.test/p.png','data:image/png;base64,AA','images/missing.png','images/a.svg']){
   await assert.rejects(resolveContentImage(root,{file:'教程.md',line:5,href}),ContentFormatError)
  }
  await symlink(path.join(root,'images'),path.join(root,'linked'))
  await assert.rejects(resolveContentImage(root,{file:'教程.md',line:5,href:'linked/sample.png'}),/符号链接/)
  await symlink(path.join(root,'教程.md'),path.join(root,'链接.md'))
  await assert.rejects(checkContentDirectory(root),/符号链接/)
 }finally{await rm(root,{recursive:true,force:true})}
})
test('CLI success and error output never print standard answers',async()=>{
 const root=await mkdtemp(path.join(os.tmpdir(),'content-cli-'))
 const cli=new URL('../server/scripts/check-content.mjs',import.meta.url)
 try{
  await writeFile(path.join(root,'教程.md'),module('# A\n\n'+quiz()))
  const success=spawnSync(process.execPath,['--experimental-strip-types',cli.pathname,root],{encoding:'utf8'})
  assert.equal(success.status,0,success.stderr);assert.equal(JSON.parse(success.stdout).modules[0].questionCount,1)
  assert.ok(!success.stdout.includes('secret-standard-answer'))
  await writeFile(path.join(root,'教程.md'),module('# A\n\n'+quiz('prompt: x\nanswer: [PRIVATE_BAD_ANSWER')))
  const failure=spawnSync(process.execPath,['--experimental-strip-types',cli.pathname,root],{encoding:'utf8'})
  assert.equal(failure.status,1);assert.equal(JSON.parse(failure.stderr).ok,false)
  assert.ok(!failure.stderr.includes('PRIVATE_BAD_ANSWER'))
 }finally{await rm(root,{recursive:true,force:true})}
})
