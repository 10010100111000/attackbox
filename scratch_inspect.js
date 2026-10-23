const fs = require('fs');

const html = fs.readFileSync('prototypes/learning-admin/index.html', 'utf-8');
const style = html.match(/<style>([\s\S]*?)<\/style>/)[1];

const terms = ['.chapter-list', '.chapter-item', '.panel', '.editor-tabs', '.markdown-editor', '.question-card', '.actions'];
terms.forEach(t => {
  const reg = new RegExp(`(${t}[^{]*\\{[^}]+\\})`, 'g');
  const matches = style.match(reg);
  if (matches) console.log(matches.slice(0, 2).join('\n'));
});
