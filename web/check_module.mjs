import fs from 'node:fs';
import MarkdownIt from 'markdown-it';
import { parseModuleMarkdown } from './server/content/parse-module.ts';

const content = fs.readFileSync('E:/githubdownload/attackboxmodule/module/Linux基础一.md', 'utf-8');
const lines = content.split(/\r?\n/);
const closing = lines.indexOf('---', 1);
const bodyLines = lines.slice(closing + 1);
const md = new MarkdownIt({ html: false });
const tokens = md.parse(bodyLines.join('\n'), {});
const headings = tokens.filter(t => t.type === 'heading_open' && t.tag === 'h1' && t.level === 0);
console.log('--- Headings ---');
headings.forEach(h => {
  const line = h.map[0] + closing + 2;
  const title = tokens[tokens.indexOf(h) + 1].content;
  console.log(`Line ${line}: "${title}"`);
});

try {
  const parsed = parseModuleMarkdown('Linux基础一.md', content);
  console.log('Module parsed successfully! Chapter count:', parsed.chapters.length);
  console.log('Title:', parsed.title);
  console.log('Difficulty:', parsed.difficulty);
  console.log('Summary:', parsed.summary);
  console.log('Summary length:', parsed.summary.length);
  console.log('Learning Outcomes count:', parsed.learningOutcomes.length);
  parsed.learningOutcomes.forEach((o, i) => console.log(`  ${i + 1}. ${o}`));
  console.log('Chapters details:');
  parsed.chapters.forEach(c => {
    console.log(`  - [${c.position}] ${c.title} (duration: ${c.durationMinutes} min, access: ${c.access}, questions: ${c.questions.length})`);
  });
} catch (e) {
  console.log('Parse error message:', e.message);
  console.log('Issues:', e.issues);
}
