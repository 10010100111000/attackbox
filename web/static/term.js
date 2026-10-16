/**
 * AttackBox Web 虚拟终端空间管理器
 * 支持多命令行桌面级浮动窗体、自由鼠标拖拽、边缘平滑缩放、双击全屏、ResizeObserver 自动流动排版
 */

let termCounter = 0;
let highestZ = 10;
const activeTerms = new Map(); // winId -> { id, dom, term, fit, ws, origBounds }

const canvas = document.getElementById('workspace-canvas');
const emptyHint = document.getElementById('empty-hint');

function updateEmptyHint() {
  if (emptyHint) {
    emptyHint.style.display = activeTerms.size === 0 ? 'block' : 'none';
  }
}

/**
 * 在画布内创建一个新的桌面端风格命令行窗口
 */
function createTerminalWindow() {
  termCounter++;
  const num = termCounter;
  const winId = 'win-' + num;
  const slotText = window.boxSlot || 'box';

  // 1. 创建窗体外壳 DOM (macOS 暗黑磨砂风格)
  const winDom = document.createElement('div');
  winDom.className = 'term-window focused';
  winDom.id = winId;
  highestZ++;
  winDom.style.zIndex = highestZ;

  // 初始尺寸与级联坐标 (自适应画布比例，绝不人为限制固定像素)
  const canvasW = canvas.clientWidth || window.innerWidth;
  const canvasH = canvas.clientHeight || window.innerHeight;
  const initW = Math.min(Math.max(canvasW * 0.55, 480), 860);
  const initH = Math.min(Math.max(canvasH * 0.58, 300), 520);
  const cascadeIdx = (activeTerms.size) % 6;
  const initX = Math.max(30 + cascadeIdx * 28, 10);
  const initY = Math.max(20 + cascadeIdx * 24, 10);

  winDom.style.width = `${initW}px`;
  winDom.style.height = `${initH}px`;
  winDom.style.left = `${initX}px`;
  winDom.style.top = `${initY}px`;

  winDom.innerHTML = `
    <div class="term-header">
      <div class="term-dots">
        <button class="dot-btn dot-close" title="关闭终端 (Ctrl+Shift+W)"></button>
        <button class="dot-btn dot-min" title="折叠/最小化"></button>
        <button class="dot-btn dot-max" title="最大化/还原"></button>
      </div>
      <div class="term-title">>_ 终端 #${num} [${window.boxUser || 'box'}@${slotText}]</div>
      <div class="term-header-right">
        <span class="badge-tag">PTY #${num}</span>
      </div>
    </div>
    <div class="term-body"></div>
    <div class="resize-handle resize-handle-r"></div>
    <div class="resize-handle resize-handle-b"></div>
    <div class="resize-handle resize-handle-br"></div>
  `;

  canvas.appendChild(winDom);

  // 2. 初始化 xterm.js 与 FitAddon
  const termBody = winDom.querySelector('.term-body');
  const term = new Terminal({
    cursorBlink: true,
    cursorStyle: 'block',
    scrollback: 5000,
    fontFamily: "'JetBrains Mono', 'Fira Code', ui-monospace, Menlo, Consolas, monospace",
    fontSize: 13,
    lineHeight: 1.25,
    theme: {
      background: '#09090b',
      foreground: '#e4e4e7',
      cursor: '#10b981',
      cursorAccent: '#09090b',
      selectionBackground: 'rgba(255, 255, 255, 0.18)',
      black: '#18181b', red: '#ef4444', green: '#22c55e', yellow: '#eab308',
      blue: '#3b82f6', magenta: '#a855f7', cyan: '#06b6d4', white: '#f4f4f5',
      brightBlack: '#71717a', brightRed: '#f87171', brightGreen: '#4ade80',
      brightYellow: '#fde047', brightBlue: '#60a5fa', brightMagenta: '#c084fc',
      brightCyan: '#22d3ee', brightWhite: '#ffffff'
    }
  });

  const fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open(termBody);

  // 3. 建立独立 WebSocket 字节流连接 (?win=wN 独立会话隔离)
  const scheme = location.protocol === 'https:' ? 'wss://' : 'ws://';
  const ws = new WebSocket(`${scheme}${location.host}/term/ws?win=w${num}`);
  ws.binaryType = 'arraybuffer';

  function syncSize() {
    if (ws && ws.readyState === WebSocket.OPEN && term.cols > 0 && term.rows > 0) {
      ws.send(JSON.stringify({ cols: term.cols, rows: term.rows }));
    }
  }

  ws.onopen = () => {
    try { fit.fit(); } catch (e) {}
    syncSize();
    term.focus();
  };

  ws.onmessage = (e) => {
    if (typeof e.data !== 'string') {
      term.write(new Uint8Array(e.data));
    }
  };

  ws.onclose = (e) => {
    if (e.code === 4001) {
      term.write('\r\n\x1b[33m[AttackBox] 该窗口已被新会话替换接管。\x1b[0m\r\n');
    } else {
      term.write('\r\n\x1b[90m[AttackBox] 终端连接已断开。\x1b[0m\r\n');
    }
  };

  ws.onerror = () => {
    term.write('\r\n\x1b[31m[AttackBox] 网络异常，无法连接沙箱终端。\x1b[0m\r\n');
  };

  term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(new TextEncoder().encode(data));
    }
  });

  // 4. ResizeObserver: 纯自动化流式自适应，无需开发者手工计算任何高宽
  const resizeObserver = new ResizeObserver(() => {
    try {
      fit.fit();
      syncSize();
    } catch (e) {}
  });
  resizeObserver.observe(termBody);

  // 5. 窗口交互逻辑：置顶聚焦、拖动移动、拉伸调节、红黄绿按钮
  function focusWindow() {
    document.querySelectorAll('.term-window').forEach(w => w.classList.remove('focused'));
    winDom.classList.add('focused');
    highestZ++;
    winDom.style.zIndex = highestZ;
    term.focus();
  }

  winDom.addEventListener('pointerdown', focusWindow);

  // 拖动标题栏
  const header = winDom.querySelector('.term-header');
  let isDragging = false;
  let startX = 0, startY = 0, initLeft = 0, initTop = 0;

  header.addEventListener('pointerdown', (e) => {
    if (e.target.closest('.dot-btn')) return; // 避免三色按钮误触
    isDragging = true;
    startX = e.clientX;
    startY = e.clientY;
    initLeft = winDom.offsetLeft;
    initTop = winDom.offsetTop;
    header.setPointerCapture(e.pointerId);
  });

  header.addEventListener('pointermove', (e) => {
    if (!isDragging) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    const maxLeft = Math.max(canvas.clientWidth - 80, 0);
    const maxTop = Math.max(canvas.clientHeight - 40, 0);
    winDom.style.left = `${Math.min(Math.max(initLeft + dx, 0), maxLeft)}px`;
    winDom.style.top = `${Math.min(Math.max(initTop + dy, 0), maxTop)}px`;
  });

  const stopDrag = () => { isDragging = false; };
  header.addEventListener('pointerup', stopDrag);
  header.addEventListener('pointercancel', stopDrag);

  // 边缘拉伸调节大小
  function bindResizeHandle(handleEl, resizeX, resizeY) {
    let isResizing = false;
    let sX = 0, sY = 0, sW = 0, sH = 0;

    handleEl.addEventListener('pointerdown', (e) => {
      e.stopPropagation();
      isResizing = true;
      sX = e.clientX;
      sY = e.clientY;
      sW = winDom.offsetWidth;
      sH = winDom.offsetHeight;
      handleEl.setPointerCapture(e.pointerId);
      focusWindow();
    });

    handleEl.addEventListener('pointermove', (e) => {
      if (!isResizing) return;
      if (resizeX) {
        const newW = Math.max(sW + (e.clientX - sX), 380);
        winDom.style.width = `${newW}px`;
      }
      if (resizeY) {
        const newH = Math.max(sH + (e.clientY - sY), 220);
        winDom.style.height = `${newH}px`;
      }
    });

    const stopResize = () => { isResizing = false; };
    handleEl.addEventListener('pointerup', stopResize);
    handleEl.addEventListener('pointercancel', stopResize);
  }

  bindResizeHandle(winDom.querySelector('.resize-handle-r'), true, false);
  bindResizeHandle(winDom.querySelector('.resize-handle-b'), false, true);
  bindResizeHandle(winDom.querySelector('.resize-handle-br'), true, true);

  // 红黄绿三圆点交互
  const closeBtn = winDom.querySelector('.dot-close');
  const minBtn = winDom.querySelector('.dot-min');
  const maxBtn = winDom.querySelector('.dot-max');

  function closeWindow() {
    resizeObserver.disconnect();
    try { ws.close(); } catch (e) {}
    try { term.dispose(); } catch (e) {}
    winDom.remove();
    activeTerms.delete(winId);
    updateEmptyHint();
  }

  closeBtn.addEventListener('click', closeWindow);

  // 折叠 / 展开窗体高度
  let isMinimized = false;
  let preMinH = '';
  minBtn.addEventListener('click', () => {
    if (!isMinimized) {
      preMinH = winDom.style.height;
      winDom.style.height = '36px';
      termBody.style.display = 'none';
      isMinimized = true;
    } else {
      winDom.style.height = preMinH;
      termBody.style.display = 'block';
      isMinimized = false;
      try { fit.fit(); } catch (e) {}
    }
  });

  // 双击标题栏或点击绿圆点：全屏最大化 / 还原
  let isMaximized = false;
  let savedBounds = null;

  function toggleMaximize() {
    if (!isMaximized) {
      savedBounds = {
        left: winDom.style.left,
        top: winDom.style.top,
        width: winDom.style.width,
        height: winDom.style.height
      };
      winDom.style.left = '4px';
      winDom.style.top = '4px';
      winDom.style.width = 'calc(100% - 8px)';
      winDom.style.height = 'calc(100% - 8px)';
      isMaximized = true;
    } else {
      if (savedBounds) {
        winDom.style.left = savedBounds.left;
        winDom.style.top = savedBounds.top;
        winDom.style.width = savedBounds.width;
        winDom.style.height = savedBounds.height;
      }
      isMaximized = false;
    }
  }

  maxBtn.addEventListener('click', toggleMaximize);
  header.addEventListener('dblclick', toggleMaximize);

  // 记录到活动窗口映射表
  activeTerms.set(winId, { winId, num, winDom, term, fit, ws, close: closeWindow });
  updateEmptyHint();
}

/**
 * 整齐瀑布流级联排列所有窗口
 */
function cascadeWindows() {
  let idx = 0;
  activeTerms.forEach(item => {
    item.winDom.style.left = `${30 + idx * 30}px`;
    item.winDom.style.top = `${20 + idx * 26}px`;
    idx++;
  });
}

/**
 * 关闭所有命令行窗口
 */
function closeAllWindows() {
  if (activeTerms.size === 0) return;
  if (!confirm('确定要关闭工作区内所有打开的命令行窗口吗？')) return;
  const list = Array.from(activeTerms.values());
  list.forEach(item => item.close());
}

// 页面初始化时，默认自动新建第一个终端窗口
window.addEventListener('DOMContentLoaded', () => {
  createTerminalWindow();
});
