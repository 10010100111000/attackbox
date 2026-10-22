// Scripted terminal atmosphere; nothing is executed on a server.
const screen=document.getElementById('terminal-screen'),motion=matchMedia('(prefers-reduced-motion: reduce)');
const scenes={
 nmap:{branch:'recon / service-discovery',command:'nmap -sV -p 22,80,445 192.0.2.10',rows:[['dim','Starting Nmap 7.95 ( https://nmap.org )'],['','Nmap scan report for lab-target (192.0.2.10)'],['','Host is up (0.0021s latency).'],['dim','PORT     STATE  SERVICE       VERSION'],['accent','22/tcp   open   ssh           OpenSSH 9.6'],['','80/tcp   open   http          nginx 1.24.0'],['accent','445/tcp  open   microsoft-ds  Samba smbd 4.19'],['dim','Service detection performed.'],['success','Nmap done: 1 IP address (1 host up) scanned']]},
 smb:{branch:'recon / smb-enumeration',command:'smbclient -L //192.0.2.10 -N',rows:[['dim','Anonymous login successful'],['','        Sharename       Type      Comment'],['dim','        ---------       ----      -------'],['accent','        public          Disk      Training files'],['','        IPC$            IPC       IPC Service'],['','        print$          Disk      Printer Drivers'],['dim','SMB1 disabled -- no workgroup available'],['success','Session complete · isolated lab target']]},
 msf:{branch:'research / module-inspection',command:'msfconsole -q -x "search type:auxiliary smb; exit"',rows:[['dim','Matching Modules'],['','   #  Name                                 Rank'],['dim','   -  ----                                 ----'],['accent','   0  auxiliary/scanner/smb/smb_version     normal'],['','   1  auxiliary/scanner/smb/smb_enumshares  normal'],['','   2  auxiliary/scanner/smb/smb_enumusers   normal'],['dim','Interact with a module by name or index.'],['success','Module search complete · no modules executed']]}
};

const keys=Object.keys(scenes);
let sceneKey='nmap',frame,disposed=false,last=0,clock=0,phaseTime=0,phase='typing',charIndex=0,rowIndex=0,commandNode,sceneCount=0;
const feed=document.createElement('div');feed.className='terminal-feed';screen.append(feed);
function append(type,text){const row=document.createElement('div');row.className=`terminal-line ${type}`;row.textContent=text;feed.append(row);return row;}
function createCommand(text=''){const row=append('command','');const prompt=document.createElement('span');prompt.className='prompt';prompt.textContent='❯ ';const code=document.createElement('span');code.className='typed-command';code.textContent=text;const cursor=document.createElement('span');cursor.className='cursor';row.append(prompt,code,cursor);return code;}
function begin(key){sceneKey=key;charIndex=rowIndex=0;phase='typing';phaseTime=clock;feed.classList.remove('is-clearing');feed.replaceChildren();append('dim','research@attackbox  ~/labs  '+scenes[key].branch);append('dim',' ');commandNode=createCommand();append('dim',' ');document.documentElement.dataset.scene=key;

}

function animate(now){if(disposed)return;frame=requestAnimationFrame(animate);if(document.hidden){last=0;return;}if(!last){last=now;return;}const dt=Math.min(now-last,60);clock+=dt*(motion.matches?.7:1);last=now;advanceShelf(now,dt);const elapsed=clock-phaseTime,scene=scenes[sceneKey];
 if(phase==='typing'){const count=Math.min(scene.command.length,Math.floor(elapsed*.027));if(count!==charIndex){charIndex=count;commandNode.textContent=scene.command.slice(0,count);}if(count===scene.command.length){phase='execute';phaseTime=clock;}}
 else if(phase==='execute'&&elapsed>=350){phase='output';phaseTime=clock;feed.querySelector('.cursor')?.remove();}
 else if(phase==='output'){const count=Math.min(scene.rows.length,Math.floor(elapsed/130)+1);while(rowIndex<count)append(...scene.rows[rowIndex++]);if(rowIndex===scene.rows.length){phase='hold';phaseTime=clock;}}
 else if(phase==='hold'&&elapsed>1300){phase='clear';phaseTime=clock;feed.classList.add('is-clearing');}
 else if(phase==='clear'&&elapsed>=260){sceneCount++;begin(keys[sceneCount%keys.length]);}
}
begin('nmap');frame=requestAnimationFrame(animate);
window.addEventListener('pagehide',e=>{cancelAnimationFrame(frame);if(!e.persisted)disposed=true;});window.addEventListener('pageshow',e=>{if(e.persisted&&!disposed){last=0;frame=requestAnimationFrame(animate);}});

// Clone once for a seamless loop; duplicates are excluded from keyboard/AT navigation.
const rail=document.querySelector('.module-rail');
const originals=[...rail.querySelectorAll('[data-module]')];
for(const card of originals){const clone=card.cloneNode(true);clone.dataset.loopCopy='';clone.setAttribute('aria-hidden','true');clone.tabIndex=-1;rail.append(clone);}
let shelfPosition=0,loopWidth=0,railHovered=false,railFocused=false,railVisible=true,resumeAt=0;
function measureShelf(){loopWidth=rail.children[originals.length].offsetLeft-rail.children[0].offsetLeft;shelfPosition=loopWidth?shelfPosition%loopWidth:0;rail.scrollLeft=shelfPosition;}
function advanceShelf(now,dt){if(!loopWidth||!railVisible||railHovered||railFocused||now<resumeAt)return;shelfPosition=(shelfPosition+dt*(motion.matches?.012:.032))%loopWidth;rail.scrollLeft=shelfPosition;}
rail.addEventListener('pointerenter',e=>{if(e.pointerType==='mouse')railHovered=true;});
rail.addEventListener('pointerleave',()=>{railHovered=false;resumeAt=performance.now()+500;});
rail.addEventListener('focusin',()=>railFocused=true);
rail.addEventListener('focusout',e=>{railFocused=rail.contains(e.relatedTarget);resumeAt=performance.now()+800;});
function manualScroll(){resumeAt=performance.now()+2800;}
rail.addEventListener('wheel',manualScroll,{passive:true});rail.addEventListener('pointerdown',manualScroll,{passive:true});rail.addEventListener('touchmove',manualScroll,{passive:true});rail.addEventListener('keydown',manualScroll);
rail.addEventListener('scroll',()=>{if(railHovered||railFocused||performance.now()<resumeAt)shelfPosition=rail.scrollLeft;},{passive:true});
const railResize=new ResizeObserver(measureShelf);railResize.observe(rail);measureShelf();
const railVisibility=new IntersectionObserver(entries=>railVisible=entries[0].isIntersecting,{rootMargin:'100px'});railVisibility.observe(rail);
window.addEventListener('pagehide',e=>{if(!e.persisted){railResize.disconnect();railVisibility.disconnect();}});
document.querySelectorAll('[data-module]').forEach(card=>card.addEventListener('pointermove',e=>{if(motion.matches)return;const r=card.getBoundingClientRect();card.style.setProperty('--mx',`${e.clientX-r.left}px`);card.style.setProperty('--my',`${e.clientY-r.top}px`);}));
