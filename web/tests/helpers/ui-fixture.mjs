import { createServer } from 'node:http'
import { readFile, mkdtemp, rm } from 'node:fs/promises'
import { execFileSync } from 'node:child_process'
import { tmpdir } from 'node:os'
import { dirname, resolve } from 'node:path'
import { createHash } from 'node:crypto'
import { fileURLToPath } from 'node:url'
import { build } from 'esbuild'
import { parse, compileScript, compileStyle } from 'vue/compiler-sfc'

const web = resolve(dirname(fileURLToPath(import.meta.url)), '../..')

// Render the real components and editor with Vue Router. Only Nuxt services
// and API responses are substituted, so these regressions require no database.
export async function uiFixture() {
  const directory = await mkdtemp(resolve(tmpdir(), 'attackbox-ui-'))
  const styles = []
  const runtime = `
    import {ref,computed} from 'vue';
    import {useRoute} from 'vue-router';
    export {ref,computed,useRoute};
    export const definePageMeta=()=>{}, useHead=()=>{}, clearNuxtData=()=>{};
    export const navigateTo=path=>window.fixtureRouter.push(path);
    export const useConfirm=()=>async()=>{window.confirmCalls++;return false};
    export async function $fetch(url,options={}) {
      const response=await fetch(url,{method:options.method||'GET',headers:{'Content-Type':'application/json'},body:options.body?JSON.stringify(options.body):undefined});
      const data=await response.json();
      if(!response.ok)throw Object.assign(new Error(data.message),{data,statusCode:response.status});
      return data;
    }
    export const useRequestFetch=()=>$fetch;
    export async function useLazyFetch(url){
      const data=ref(await $fetch(url));
      return {data,error:ref(null),pending:ref(false),refresh:async()=>{data.value=await $fetch(url)}};
    }
  `
  const entry = `
    import {createApp,h,ref,Suspense} from 'vue';
    import {createRouter,createWebHistory,RouterView,RouterLink} from 'vue-router';
    import Button from './components/ui/Button.vue';
    import Input from './components/ui/Input.vue';
    import Notice from './components/ui/Notice.vue';
    import Badge from './components/ui/Badge.vue';
    import EmptyState from './components/ui/EmptyState.vue';
    import BusyLabel from './components/ui/BusyLabel.vue';
    import BusyIndicator from './components/ui/BusyIndicator.vue';
    import Users from './pages/admin/users/index.vue';
    import References from './pages/admin/references/index.vue';
    import Routes from './pages/admin/routes/[id].vue';
    import Notifications from './pages/admin/notifications.vue';
    window.activations=0;window.confirmCalls=0;
    const button=(id,props={})=>h(Button,{id,onClick:()=>window.activations++,...props},()=>id);
    const TestComponents={setup(){
      const busy=ref(false);window.setFixtureBusy=value=>busy.value=value;
      return ()=>h('div',[
        button('outside'),
        h('div',{class:'admin-shell'},[
          h('div',{class:'users-page',[Users.__scopeId]:''},[button('users-primary'),button('users-danger',{variant:'danger'})]),
          h('div',{class:'reference-admin',[References.__scopeId]:''},[
            h('table',[h('tbody',[h('tr',[h('td',{[References.__scopeId]:''},[button('reference-primary',{to:'/destination',[References.__scopeId]:''})])])])])
          ])
        ]),
        button('internal',{to:'/destination',disabled:busy.value}),
        button('download',{href:'/download.md',download:'sample.md',disabled:busy.value}),
        button('loading-link',{href:'/destination',loading:busy.value}),
        button('native',{disabled:busy.value}),
        h('fieldset',{disabled:true},[button('fieldset-native')]),
        h(Notice,{variant:'error',id:'error-notice'},()=> 'failure'),
        h(Notice,{variant:'success',id:'success-notice'},()=> 'saved'),
        h(Input,{id:'password',type:'password',disabled:busy.value}),
      ]);
    }};
    const router=createRouter({history:createWebHistory(),routes:[
      {path:'/',component:TestComponents},
      {path:'/destination',component:{render:()=>h('p','destination')}},
      {path:'/admin/routes/:id',component:Routes},
      {path:'/admin/notifications',component:Notifications},
    ]});
    window.fixtureRouter=router;
    const app=createApp({render:()=>h('div',{class:'admin-shell'},[h(RouterView,null,{default:({Component})=>h(Suspense,null,{default:()=>h(Component)})})])});
    const NuxtLink={inheritAttrs:false,props:['to'],setup(props,{attrs,slots}){return ()=>props.to?h(RouterLink,{...attrs,to:props.to},slots):h('a',attrs,slots.default?.())}};
    for(const [name,component] of Object.entries({UiButton:Button,UiInput:Input,UiNotice:Notice,UiBadge:Badge,UiEmptyState:EmptyState,UiBusyLabel:BusyLabel,UiBusyIndicator:BusyIndicator,NuxtLink}))app.component(name,component);
    app.component('AdminDataLoading',{render:()=>null});
    app.use(router);await router.isReady();app.mount('#app');
  `
  let server
  try {
    const layout = parse(await readFile(resolve(web, 'layouts/admin.vue'), 'utf8')).descriptor
    for (const style of layout.styles.filter(style => style.src)) {
      styles.push(await readFile(resolve(web, style.src.replace('~/', '')), 'utf8'))
    }
    const result = await build({
      stdin: { contents: entry, resolveDir: web }, bundle: true, write: false,
      format: 'esm', alias: { '~': web },
      plugins: [{ name: 'vue-fixture', setup(builder) {
        builder.onResolve({ filter: /^audit-runtime$/ }, () => ({ path: 'runtime', namespace: 'fixture' }))
        builder.onLoad({ filter: /.*/, namespace: 'fixture' }, () => ({ contents: runtime, resolveDir: web }))
        builder.onLoad({ filter: /\.vue$/ }, async ({ path }) => {
          const { descriptor } = parse(await readFile(path, 'utf8'), { filename: path })
          const id = 'data-v-' + createHash('sha256').update(path).digest('hex').slice(0, 8)
          for (const style of descriptor.styles) {
            let source = style.src ? await readFile(resolve(web, style.src.replace('~/', '')), 'utf8') : style.content
            for (const match of source.matchAll(/@import ['"]~\/(.*?)['"];?/g)) {
              source = source.replace(match[0], await readFile(resolve(web, match[1]), 'utf8'))
            }
            const compiled = compileStyle({ source, filename: path, id, scoped: !!style.scoped })
            if (compiled.errors.length) throw compiled.errors[0]
            styles.push(compiled.code)
          }
          let code = compileScript(descriptor, { id, inlineTemplate: true }).content
          if (path.includes('/pages/admin/')) {
            code = `import {definePageMeta,useHead,useLazyFetch,useRequestFetch,useRoute,useConfirm,clearNuxtData,navigateTo,$fetch} from 'audit-runtime';\n` + code
            if (path.endsWith('/notifications.vue')) code = `import {ref,computed} from 'vue';\n` + code
          }
          if (descriptor.styles.some(style => style.scoped)) {
            code = code.replace('export default', 'const component =') + `\ncomponent.__scopeId='${id}';export default component;`
          }
          return { contents: code, loader: 'ts', resolveDir: dirname(path) }
        })
      } }],
    })
    const cssFile = resolve(directory, 'style.css')
    execFileSync(process.execPath, [resolve(web, 'node_modules/tailwindcss/lib/cli.js'), '-i', 'assets/css/main.css', '-c', 'tailwind.config.ts', '-o', cssFile], { cwd: web, stdio: 'pipe' })
    const css = await readFile(cssFile, 'utf8') + '\n' + styles.join('\n')
    const fixtures = {
      '/api/admin/modules': { data: [{ id: 'module-one', title: 'Module one', status: 'published' }] },
      '/api/admin/routes/existing': { data: { title: 'Original route', summary: '', modules: [], lockVersion: 0, status: 'unpublished' } },
      '/api/admin/notifications': { data: { items: [], total: 0 } },
    }
    server = createServer((req, res) => {
      const url = new URL(req.url, 'http://localhost')
      if (url.pathname === '/bundle.js') { res.setHeader('Content-Type', 'text/javascript');res.end(result.outputFiles[0].text) }
      else if (url.pathname === '/style.css') { res.setHeader('Content-Type', 'text/css');res.end(css) }
      else if (url.pathname.startsWith('/api/')) { res.setHeader('Content-Type', 'application/json');res.end(JSON.stringify(fixtures[url.pathname] || {})) }
      else res.end('<!doctype html><html data-theme="graphite"><head><link rel="stylesheet" href="/style.css"></head><body><div id="app"></div><script type="module" src="/bundle.js"></script></body></html>')
    })
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
    return { base: `http://127.0.0.1:${server.address().port}`, close: async () => { await new Promise(resolve => server.close(resolve));await rm(directory, { recursive: true, force: true }) } }
  } catch (error) {
    server?.close()
    await rm(directory, { recursive: true, force: true })
    throw error
  }
}
