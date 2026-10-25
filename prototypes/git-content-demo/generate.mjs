// Standalone authoring experiment. Not connected to the application or database.
import {readFile,writeFile} from 'node:fs/promises'
import {fileURLToPath} from 'node:url'
import path from 'node:path'
import {createRequire} from 'node:module'
const require=createRequire(new URL('../../web/package.json',import.meta.url))
const MarkdownIt=require('markdown-it'),YAML=require('yaml')
const sourcePath=fileURLToPath(new URL('./Linux 基础.md',import.meta.url))
const source=await readFile(sourcePath,'utf8')
const header=source.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/)
if(!header)throw Error('Missing YAML frontmatter')
const metadata=YAML.parse(header[1])
const body=source.slice(header[0].length),lines=body.split('\n')
const md=new MarkdownIt({html:false,linkify:false})
const tokens=md.parse(body,{})
const headings=tokens.filter(t=>t.type==='heading_open'&&t.tag==='h1'&&t.level===0)
if(!headings.length)throw Error('At least one # chapter heading is required')
if(lines.slice(0,headings[0].map[0]).join('\n').trim())throw Error('Chapter content must begin with a # heading')
const title=path.basename(sourcePath,'.md')
const chapters=headings.map((heading,index)=>{
 const start=heading.map[0],end=headings[index+1]?.map[0]??lines.length
 const titleToken=tokens[tokens.indexOf(heading)+1]
 const quizzes=tokens.filter(t=>t.type==='fence'&&t.level===0&&t.info.trim()==='quiz'&&t.map[0]>=start&&t.map[0]<end)
 const questions=quizzes.map(t=>{
  const q=YAML.parse(t.content)
  if(typeof q.prompt!=='string'||typeof q.answer!=='string'||!q.answer.trim())throw Error(`Invalid quiz in ${titleToken.content}`)
  if(!['exact','trim','trim_casefold'].includes(q.normalization??'trim'))throw Error('Invalid answer normalization')
  return {...q,normalization:q.normalization??'trim'}
 })
 // Remove quiz source before rendering; answers never enter the preview HTML.
 const prose=lines.slice(heading.map[1],end).filter((_,offset)=>!quizzes.some(t=>offset+heading.map[1]>=t.map[0]&&offset+heading.map[1]<t.map[1])).join('\n')
 return {title:titleToken.content,position:index+1,body:md.render(prose),questions}
})
if(new Set(chapters.map(c=>c.title)).size!==chapters.length)throw Error('Duplicate chapter titles')
const esc=s=>String(s??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))
const html=`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${esc(title)} · Markdown 解析预览</title>
<style>
*{box-sizing:border-box}body{margin:0;background:#f6f7f6;color:#202a26;font:15px/1.85 system-ui,-apple-system,"Noto Sans CJK SC",sans-serif}button,input{font:inherit}header{height:64px;display:flex;align-items:center;justify-content:space-between;padding:0 32px;border-bottom:1px solid #e0e5e1;background:#fff}header strong{letter-spacing:-.5px;font-size:18px}header span{color:#77837b;font-size:12px}.layout{max-width:1360px;margin:auto;display:grid;grid-template-columns:240px minmax(0,1fr);min-height:calc(100vh - 64px)}aside{padding:36px 24px;border-right:1px solid #e0e5e1}aside small{font-size:10px;letter-spacing:2px;color:#809087}aside h2{font-size:19px;line-height:1.5;margin:8px 0 24px}nav button{display:flex;gap:12px;width:100%;border:0;border-radius:7px;padding:11px 12px;margin:4px 0;text-align:left;background:transparent;color:#67756c;cursor:pointer;font-size:13px}nav button.active{background:#e5eee8;color:#245942;font-weight:600}nav button span{font:11px/2 monospace;color:#8b978f}.module-info{font-size:12px;color:#77837b;border-top:1px solid #e0e5e1;margin-top:28px;padding-top:20px}.module-info p{margin:0 0 14px}.module-info ul{padding-left:18px;margin:0}.module-info summary{cursor:pointer;color:#43554a}main{padding:38px 64px 64px;min-width:0}section[hidden]{display:none}.meta{font:11px monospace;letter-spacing:1px;color:#839188}h1{font-size:32px;letter-spacing:-1px;line-height:1.5;margin:12px 0 28px}article{font-size:16px}article h2{font-size:21px;margin:30px 0 12px}article p{margin:14px 0}pre{background:#19251f;color:#dbe9e0;border-radius:8px;padding:16px 20px;overflow:auto;font:13px/1.8 ui-monospace,monospace}code{font-family:ui-monospace,monospace}p code,td code{background:#e9eee9;padding:2px 5px;border-radius:3px;font-size:13px}blockquote{border-left:3px solid #7a9d87;background:#edf2ee;padding:2px 18px;margin:22px 0;color:#617367}table{border-collapse:collapse;width:100%;font-size:14px}td,th{border-bottom:1px solid #dfe5df;padding:10px;text-align:left}.quiz{border-top:1px solid #dfe5df;margin-top:36px;padding-top:24px}.quiz-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px}.quiz-head h2{font-size:20px;margin:0}.quiz-head span{font-size:11px;color:#89968d}.question{font-size:15px;margin:20px 0}.question p{margin:0 0 12px}.answer{display:flex;gap:10px}.answer input{min-width:0;flex:1;border:1px solid #d7dfd8;border-radius:6px;padding:9px 12px;background:#fff;font-size:13px}.answer button{border:1px solid #d7dfd8;border-radius:6px;padding:8px 15px;background:#fff;color:#8c978f;font-size:12px}.footer{border-top:1px solid #dfe5df;margin-top:28px;padding-top:20px;display:flex;justify-content:flex-end}.footer button{border:0;background:#2f6650;color:#fff;border-radius:6px;padding:10px 18px;font-size:12px;cursor:pointer}button:focus-visible{outline:2px solid #2f6650;outline-offset:3px}@media(max-width:800px){.layout{grid-template-columns:190px minmax(0,1fr)}main{padding:28px}aside{padding:28px 16px}} 
</style><header><strong>AttackBox</strong><span>Markdown 解析预览 · ${chapters.length} 章 · ${chapters.reduce((n,c)=>n+c.questions.length,0)} 道练习</span></header><div class="layout"><aside><small>LEARNING MODULE</small><h2>${esc(title)}</h2><nav aria-label="章节目录">${chapters.map((c,i)=>`<button data-chapter="${i}" class="${i===0?'active':''}"><span>${String(i+1).padStart(2,'0')}</span>${esc(c.title)}</button>`).join('')}</nav><div class="module-info"><p>${esc(metadata.summary)}</p><details><summary>模块介绍与目标</summary><p>${esc(metadata.description)}</p><ul>${(metadata.learningOutcomes??[]).map(g=>`<li>${esc(g)}</li>`).join('')}</ul></details></div></aside><main>${chapters.map((c,i)=>`<section id="chapter-${i}" ${i?'hidden':''}><div class="meta">CHAPTER ${String(i+1).padStart(2,'0')} / ${String(chapters.length).padStart(2,'0')}</div><h1>${esc(c.title)}</h1><article>${c.body}</article><div class="quiz"><div class="quiz-head"><h2>章节练习</h2><span>预览不执行判题</span></div>${c.questions.map((q,j)=>`<div class="question"><p>${j+1}. ${esc(q.prompt)}</p><div class="answer"><input aria-label="第 ${j+1} 题答案" placeholder="输入你的答案"><button disabled>校验答案</button></div></div>`).join('')}</div><div class="footer">${i<chapters.length-1?`<button data-chapter="${i+1}">${esc(chapters[i+1].title)} →</button>`:'<span style="font-size:12px;color:#89968d">解析预览结束</span>'}</div></section>`).join('')}</main></div><script>
document.querySelectorAll('[data-chapter]').forEach(button=>button.addEventListener('click',()=>{const index=button.dataset.chapter;document.querySelectorAll('main>section').forEach((section,i)=>section.hidden=String(i)!==index);document.querySelectorAll('nav button').forEach(item=>{const active=item.dataset.chapter===index;item.classList.toggle('active',active);item.setAttribute('aria-current',active?'step':'false')});window.scrollTo(0,0)}))
</script></html>`
await writeFile(new URL('./preview.html',import.meta.url),html)
console.log(JSON.stringify({module:title,metadata,chapters:chapters.map(c=>({title:c.title,position:c.position,questionCount:c.questions.length,normalizations:c.questions.map(q=>q.normalization)})),preview:'prototypes/git-content-demo/preview.html'},null,2))
