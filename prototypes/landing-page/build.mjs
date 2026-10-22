import { build } from 'esbuild';
import { readFile, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
const root = path.dirname(fileURLToPath(import.meta.url));
const result = await build({ entryPoints: [path.join(root, 'src/scene.js')], bundle: true, write: false, minify: true, legalComments: 'inline', format: 'iife', target: ['es2020'] });
let template = await readFile(path.join(root, 'src/index.template.html'), 'utf8');
for (const name of ['linux', 'network', 'web']) {
  const bytes = await readFile(path.join(root, 'assets', `${name}.jpg`));
  template = template.replaceAll(`__IMAGE_${name.toUpperCase()}__`, `data:image/jpeg;base64,${bytes.toString('base64')}`);
}
const code = result.outputFiles[0].text.replace(/<\/script/gi, '<\\/script');
await writeFile(path.join(root, 'index.html'), template.replace('/* __SCENE_BUNDLE__ */', () => code));
console.log('Built standalone index.html (scripts and photos embedded).');
