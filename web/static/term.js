/**
 * AttackBox Web 虚拟终端空间交互核心
 * - 深度统一控制台 Dashboard 设计语言
 * - 类似 Windows Aero Snap 智能贴靠 (拖至顶部全屏、左右半屏、下拉还原)
 * - Win7 Flip 3D (3D 旋转层叠、鼠标滚轮流畅翻页、点击选中切换)
 * - 自由拖拽定位的快捷记事本 (跨 GUI/CLI 共享 Payload 库、一键键入标准输入)
 */

let termCounter = 0;
let highestZ = 10;
const activeTerms = new Map(); // winId -> { id, num, dom, term, fit, ws, origRect, isSnapped, snapMode }
let focusedWinId = null;

// 3D Flip 状态
let isFlip3D = false;
let flipIndex = 0;

const canvas = document.getElementById('workspace-canvas');
const emptyHint = document.getElementById('empty-hint');
const snapPreview = document.getElementById('snap-preview');
const flipHud = document.getElementById('flip-hud');
const flipCounter = document.getElementById('flip-counter');
const termToast = document.getElementById('term-toast');
let toastTimer = null;

function showToast(msg) {
  if (toastTimer) clearTimeout(toastTimer);
  termToast.textContent = msg;
  termToast.classList.add('show');
  toastTimer = setTimeout(() => {
    termToast.classList.remove('show');
  }, 1800);
}

function updateEmptyHint() {
  if (emptyHint) {
    emptyHint.style.display = activeTerms.size === 0 ? 'block' : 'none';
  }
}

// -------------------------------------------------------------
// 1. 终端窗口创建与生命周期管理
// -------------------------------------------------------------

function createTerminalWindow() {
  if (isFlip3D) exitFlip3D(false);

  termCounter++;
  const num = termCounter;
  const winId = 'win-' + num;
  const slotText = window.boxSlot || 'box';

  const winDom = document.createElement('div');
  winDom.className = 'term-window focused';
  winDom.id = winId;
  highestZ++;
  winDom.style.zIndex = highestZ;

  // 自适应尺寸与级联坐标
  const canvasW = canvas.clientWidth || window.innerWidth;
  const canvasH = canvas.clientHeight || window.innerHeight;
  const initW = Math.min(Math.max(canvasW * 0.56, 520), 860);
  const initH = Math.min(Math.max(canvasH * 0.58, 320), 540);
  const cascadeIdx = (activeTerms.size) % 8;
  const initX = Math.max(40 + cascadeIdx * 30, 16);
  const initY = Math.max(64 + cascadeIdx * 24, 60);

  winDom.style.width = `${initW}px`;
  winDom.style.height = `${initH}px`;
  winDom.style.left = `${initX}px`;
  winDom.style.top = `${initY}px`;

  winDom.innerHTML = `
    <div class="term-header">
      <div class="term-dots">
        <button class="dot-btn dot-close" title="关闭终端 (Ctrl+Shift+W)"></button>
        <button class="dot-btn dot-min" title="最小化/恢复"></button>
        <button class="dot-btn dot-max" title="最大化/还原 (支持拖拽到顶部贴靠)"></button>
      </div>
      <div class="term-title">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#10b981" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="4 17 10 11 4 5"></polyline>
          <line x1="12" y1="19" x2="20" y2="19"></line>
        </svg>
        <span>终端 #${num} [${window.boxUser || 'box'}@${slotText}]</span>
      </div>
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

  // 初始化 xterm.js 与 FitAddon
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

  // 建立独立 WebSocket 字节流连接 (?win=wN 独立会话隔离)
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
      term.write('\r\n\x1b[90m[AttackBox] 终端会话已断开。\x1b[0m\r\n');
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

  // ResizeObserver 流式自适应排版
  const resizeObserver = new ResizeObserver(() => {
    if (isFlip3D) return;
    try {
      fit.fit();
      syncSize();
    } catch (e) {}
  });
  resizeObserver.observe(termBody);

  const termEntry = {
    id: winId,
    num: num,
    dom: winDom,
    term: term,
    fit: fit,
    ws: ws,
    origRect: { left: initX, top: initY, width: initW, height: initH },
    isSnapped: false,
    snapMode: null
  };
  activeTerms.set(winId, termEntry);
  updateEmptyHint();

  // 窗口聚焦
  function focusThis() {
    if (isFlip3D) {
      selectFlip3D(winId);
      return;
    }
    document.querySelectorAll('.term-window').forEach(w => w.classList.remove('focused'));
    winDom.classList.add('focused');
    highestZ++;
    winDom.style.zIndex = highestZ;
    focusedWinId = winId;
    term.focus();
  }
  winDom.addEventListener('mousedown', focusThis);
  focusThis();

  // -------------------------------------------------------------
  // 2. 类似 Windows Aero Snap 贴靠与标题栏拖拽逻辑
  // -------------------------------------------------------------
  const header = winDom.querySelector('.term-header');
  let isMoving = false;
  let moveStartX = 0, moveStartY = 0;
  let winStartX = 0, winStartY = 0;
  let currentSnapTarget = null;

  const SNAP_TRIGGER_MARGIN = 16;
  const TOP_OFFSET = 60; // 避开顶部悬浮 HUD 栏
  const MARGIN = 12;

  function calculateSnapRect(mode) {
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    if (mode === 'top') {
      return {
        left: MARGIN,
        top: TOP_OFFSET,
        width: vw - MARGIN * 2,
        height: vh - TOP_OFFSET - MARGIN
      };
    } else if (mode === 'left') {
      return {
        left: MARGIN,
        top: TOP_OFFSET,
        width: Math.floor((vw - MARGIN * 3) / 2),
        height: vh - TOP_OFFSET - MARGIN
      };
    } else if (mode === 'right') {
      const halfW = Math.floor((vw - MARGIN * 3) / 2);
      return {
        left: MARGIN * 2 + halfW,
        top: TOP_OFFSET,
        width: halfW,
        height: vh - TOP_OFFSET - MARGIN
      };
    }
    return null;
  }

  function showSnapPreview(mode) {
    const rect = calculateSnapRect(mode);
    if (!rect) {
      snapPreview.classList.remove('show');
      currentSnapTarget = null;
      return;
    }
    currentSnapTarget = mode;
    snapPreview.style.left = rect.left + 'px';
    snapPreview.style.top = rect.top + 'px';
    snapPreview.style.width = rect.width + 'px';
    snapPreview.style.height = rect.height + 'px';
    snapPreview.classList.add('show');
  }

  function hideSnapPreview() {
    snapPreview.classList.remove('show');
    currentSnapTarget = null;
  }

  function applySnap(mode) {
    const rect = calculateSnapRect(mode);
    if (!rect) return;

    if (!termEntry.isSnapped) {
      termEntry.origRect = {
        left: winDom.offsetLeft,
        top: winDom.offsetTop,
        width: winDom.offsetWidth,
        height: winDom.offsetHeight
      };
    }

    winDom.classList.add('snapped');
    winDom.style.left = rect.left + 'px';
    winDom.style.top = rect.top + 'px';
    winDom.style.width = rect.width + 'px';
    winDom.style.height = rect.height + 'px';

    termEntry.isSnapped = true;
    termEntry.snapMode = mode;

    setTimeout(() => {
      try {
        fit.fit();
        syncSize();
      } catch (e) {}
    }, 200);
  }

  function unSnap(mouseX, mouseY) {
    if (!termEntry.isSnapped) return;
    winDom.classList.remove('snapped');
    const orig = termEntry.origRect || { width: 640, height: 420 };
    winDom.style.width = orig.width + 'px';
    winDom.style.height = orig.height + 'px';

    // 鼠标在标题栏居中
    const newLeft = Math.max(MARGIN, Math.min(mouseX - orig.width / 2, window.innerWidth - orig.width - MARGIN));
    const newTop = Math.max(MARGIN, mouseY - 18);
    winDom.style.left = newLeft + 'px';
    winDom.style.top = newTop + 'px';

    termEntry.isSnapped = false;
    termEntry.snapMode = null;

    winStartX = newLeft;
    winStartY = newTop;
    moveStartX = mouseX;
    moveStartY = mouseY;

    setTimeout(() => {
      try {
        fit.fit();
        syncSize();
      } catch (e) {}
    }, 100);
  }

  header.addEventListener('pointerdown', (e) => {
    if (e.target.closest('.dot-btn')) return;
    if (isFlip3D) return;
    if (e.button !== 0) return;

    isMoving = true;
    moveStartX = e.clientX;
    moveStartY = e.clientY;
    winStartX = winDom.offsetLeft;
    winStartY = winDom.offsetTop;
    header.setPointerCapture(e.pointerId);
    focusThis();
  });

  header.addEventListener('pointermove', (e) => {
    if (!isMoving) return;

    const deltaX = e.clientX - moveStartX;
    const deltaY = e.clientY - moveStartY;

    // 若当前是已贴靠状态，且用户向下拉拽，触发平滑脱扣还原
    if (termEntry.isSnapped && (Math.abs(deltaX) > 8 || deltaY > 8)) {
      unSnap(e.clientX, e.clientY);
      return;
    }

    const newX = winStartX + deltaX;
    const newY = winStartY + deltaY;
    winDom.style.left = `${newX}px`;
    winDom.style.top = `${newY}px`;

    // 探测屏幕边缘触发贴靠预览
    const curX = e.clientX;
    const curY = e.clientY;

    if (curY <= SNAP_TRIGGER_MARGIN) {
      showSnapPreview('top');
    } else if (curX <= SNAP_TRIGGER_MARGIN) {
      showSnapPreview('left');
    } else if (curX >= window.innerWidth - SNAP_TRIGGER_MARGIN) {
      showSnapPreview('right');
    } else {
      hideSnapPreview();
    }
  });

  function stopMoving(e) {
    if (!isMoving) return;
    isMoving = false;
    try { header.releasePointerCapture(e.pointerId); } catch (_) {}

    if (currentSnapTarget) {
      applySnap(currentSnapTarget);
      hideSnapPreview();
    } else {
      if (!termEntry.isSnapped) {
        termEntry.origRect = {
          left: winDom.offsetLeft,
          top: winDom.offsetTop,
          width: winDom.offsetWidth,
          height: winDom.offsetHeight
        };
      }
    }
  }

  header.addEventListener('pointerup', stopMoving);
  header.addEventListener('pointercancel', stopMoving);

  // 双击标题栏最大化 / 还原
  header.addEventListener('dblclick', (e) => {
    if (e.target.closest('.dot-btn')) return;
    if (termEntry.isSnapped && termEntry.snapMode === 'top') {
      const orig = termEntry.origRect;
      winDom.classList.remove('snapped');
      winDom.style.left = orig.left + 'px';
      winDom.style.top = orig.top + 'px';
      winDom.style.width = orig.width + 'px';
      winDom.style.height = orig.height + 'px';
      termEntry.isSnapped = false;
      termEntry.snapMode = null;
    } else {
      applySnap('top');
    }
  });

  // -------------------------------------------------------------
  // 3. 窗体边缘拉伸缩放
  // -------------------------------------------------------------
  function setupResizer(handleEl, direction) {
    let isResizing = false;
    let rStartX = 0, rStartY = 0;
    let rStartW = 0, rStartH = 0;

    handleEl.addEventListener('pointerdown', (e) => {
      if (e.button !== 0 || isFlip3D) return;
      isResizing = true;
      rStartX = e.clientX;
      rStartY = e.clientY;
      rStartW = winDom.offsetWidth;
      rStartH = winDom.offsetHeight;
      handleEl.setPointerCapture(e.pointerId);
      e.stopPropagation();
      focusThis();
    });

    handleEl.addEventListener('pointermove', (e) => {
      if (!isResizing) return;
      if (termEntry.isSnapped) {
        winDom.classList.remove('snapped');
        termEntry.isSnapped = false;
        termEntry.snapMode = null;
      }
      if (direction.includes('r')) {
        const nextW = Math.max(380, rStartW + (e.clientX - rStartX));
        winDom.style.width = `${nextW}px`;
      }
      if (direction.includes('b')) {
        const nextH = Math.max(220, rStartH + (e.clientY - rStartY));
        winDom.style.height = `${nextH}px`;
      }
    });

    function stopResize(e) {
      if (!isResizing) return;
      isResizing = false;
      try { handleEl.releasePointerCapture(e.pointerId); } catch (_) {}
      termEntry.origRect = {
        left: winDom.offsetLeft,
        top: winDom.offsetTop,
        width: winDom.offsetWidth,
        height: winDom.offsetHeight
      };
      try {
        fit.fit();
        syncSize();
      } catch (err) {}
    }

    handleEl.addEventListener('pointerup', stopResize);
    handleEl.addEventListener('pointercancel', stopResize);
  }

  setupResizer(winDom.querySelector('.resize-handle-r'), 'r');
  setupResizer(winDom.querySelector('.resize-handle-b'), 'b');
  setupResizer(winDom.querySelector('.resize-handle-br'), 'rb');

  // -------------------------------------------------------------
  // 4. 红黄绿按钮控制
  // -------------------------------------------------------------
  const closeBtn = winDom.querySelector('.dot-close');
  const minBtn = winDom.querySelector('.dot-min');
  const maxBtn = winDom.querySelector('.dot-max');

  closeBtn.onclick = (e) => {
    e.stopPropagation();
    destroyTerminalWindow(winId);
  };

  maxBtn.onclick = (e) => {
    e.stopPropagation();
    if (termEntry.isSnapped && termEntry.snapMode === 'top') {
      const orig = termEntry.origRect;
      winDom.classList.remove('snapped');
      winDom.style.left = orig.left + 'px';
      winDom.style.top = orig.top + 'px';
      winDom.style.width = orig.width + 'px';
      winDom.style.height = orig.height + 'px';
      termEntry.isSnapped = false;
      termEntry.snapMode = null;
    } else {
      applySnap('top');
    }
  };

  let isFolded = false;
  minBtn.onclick = (e) => {
    e.stopPropagation();
    isFolded = !isFolded;
    if (isFolded) {
      winDom.dataset.prevHeight = winDom.style.height;
      winDom.style.height = '36px';
      winDom.querySelector('.term-body').style.display = 'none';
    } else {
      winDom.style.height = winDom.dataset.prevHeight || '400px';
      winDom.querySelector('.term-body').style.display = 'block';
      try { fit.fit(); } catch (_) {}
    }
  };

  return termEntry;
}

function destroyTerminalWindow(winId) {
  const item = activeTerms.get(winId);
  if (!item) return;

  try {
    if (item.ws) {
      item.ws.onclose = null;
      item.ws.close();
    }
    if (item.term) item.term.dispose();
  } catch (e) {}

  if (item.dom && item.dom.parentNode) {
    item.dom.parentNode.removeChild(item.dom);
  }

  activeTerms.delete(winId);
  if (focusedWinId === winId) {
    focusedWinId = null;
    const remaining = Array.from(activeTerms.keys());
    if (remaining.length > 0) {
      const nextId = remaining[remaining.length - 1];
      const nextTerm = activeTerms.get(nextId);
      if (nextTerm) {
        nextTerm.dom.classList.add('focused');
        focusedWinId = nextId;
        nextTerm.term.focus();
      }
    }
  }

  updateEmptyHint();
  if (isFlip3D) {
    if (activeTerms.size === 0) exitFlip3D(false);
    else renderFlip3D();
  }
}

function closeAllWindows() {
  if (activeTerms.size === 0) return;
  const ids = Array.from(activeTerms.keys());
  ids.forEach(id => destroyTerminalWindow(id));
  showToast('所有终端窗口已关闭');
}

// -------------------------------------------------------------
// 5. 排列功能：Win7 Flip 3D (3D 旋转层叠与鼠标滚轮翻页)
// -------------------------------------------------------------

function toggleFlip3D() {
  if (isFlip3D) {
    exitFlip3D(true);
  } else {
    enterFlip3D();
  }
}

function enterFlip3D() {
  const arr = Array.from(activeTerms.values());
  if (arr.length === 0) {
    showToast('当前工作区暂无终端窗口，请先新建');
    return;
  }

  isFlip3D = true;
  flipIndex = 0;
  if (focusedWinId) {
    const idx = arr.findIndex(t => t.id === focusedWinId);
    if (idx !== -1) flipIndex = idx;
  }

  canvas.classList.add('flip-3d');
  document.getElementById('hud-island').classList.add('hidden-hud');
  flipHud.classList.add('show');

  renderFlip3D();
}

function exitFlip3D(applyFocus = true) {
  if (!isFlip3D) return;
  isFlip3D = false;

  canvas.classList.remove('flip-3d');
  document.getElementById('hud-island').classList.remove('hidden-hud');
  flipHud.classList.remove('show');

  const arr = Array.from(activeTerms.values());
  arr.forEach(item => {
    item.dom.classList.remove('in-3d');
    item.dom.style.transform = '';
    item.dom.style.opacity = '';
    item.dom.style.filter = '';
    item.dom.style.boxShadow = '';
    item.dom.style.zIndex = '';
  });

  if (applyFocus && arr.length > 0) {
    const chosen = arr[flipIndex] || arr[0];
    if (chosen) {
      document.querySelectorAll('.term-window').forEach(w => w.classList.remove('focused'));
      chosen.dom.classList.add('focused');
      highestZ++;
      chosen.dom.style.zIndex = highestZ;
      focusedWinId = chosen.id;
      setTimeout(() => {
        try { chosen.fit.fit(); } catch (_) {}
        chosen.term.focus();
      }, 250);
    }
  }
}

function renderFlip3D() {
  const arr = Array.from(activeTerms.values());
  const count = arr.length;
  if (count === 0) {
    exitFlip3D(false);
    return;
  }

  if (flipIndex < 0) flipIndex = 0;
  if (flipIndex >= count) flipIndex = count - 1;

  flipCounter.textContent = `${flipIndex + 1} / ${count} - 终端 #${arr[flipIndex].num}`;

  arr.forEach((item, idx) => {
    const delta = idx - flipIndex;
    const dom = item.dom;
    dom.classList.add('in-3d');

    if (delta === 0) {
      // 当前选中的前景主窗口 (居中放大展示，带翡翠绿光晕)
      dom.style.transform = `translate3d(0, 0, 0) rotateY(-34deg) rotateX(6deg) scale(1.02)`;
      dom.style.zIndex = '1000';
      dom.style.opacity = '1';
      dom.style.filter = 'brightness(1.05)';
      dom.style.boxShadow = '0 30px 60px rgba(0, 0, 0, 0.85), 0 0 35px rgba(16, 185, 129, 0.4)';
      dom.classList.add('focused');
    } else if (delta > 0) {
      // 队列中等待查看的后续窗口 (向右上方 3D 纵深阶梯延伸)
      const offX = delta * 65;
      const offY = -delta * 22;
      const offZ = -delta * 170;
      const op = Math.max(0.2, 1 - delta * 0.18);
      dom.style.transform = `translate3d(${offX}px, ${offY}px, ${offZ}px) rotateY(-34deg) rotateX(6deg) scale(${Math.max(0.7, 1 - delta * 0.04)})`;
      dom.style.zIndex = String(1000 - delta);
      dom.style.opacity = String(op);
      dom.style.filter = 'brightness(0.6) blur(0.5px)';
      dom.style.boxShadow = '0 15px 35px rgba(0, 0, 0, 0.6)';
      dom.classList.remove('focused');
    } else {
      // 已经翻阅过去的左侧离场窗口 (向左下方划出视口)
      const offX = delta * 120;
      const offY = -delta * 28;
      const offZ = delta * 120;
      dom.style.transform = `translate3d(${offX}px, ${offY}px, ${offZ}px) rotateY(-42deg) rotateX(8deg) scale(0.9)`;
      dom.style.zIndex = String(1000 + delta);
      dom.style.opacity = '0.15';
      dom.style.filter = 'brightness(0.35) blur(1px)';
      dom.classList.remove('focused');
    }
  });
}

function selectFlip3D(winId) {
  const arr = Array.from(activeTerms.values());
  const idx = arr.findIndex(t => t.id === winId);
  if (idx !== -1) {
    flipIndex = idx;
    exitFlip3D(true);
  }
}

// 鼠标滚轮切换 3D 层叠
window.addEventListener('wheel', (e) => {
  if (!isFlip3D) return;
  e.preventDefault();
  const arr = Array.from(activeTerms.values());
  if (arr.length <= 1) return;

  if (e.deltaY > 0) {
    flipIndex = (flipIndex + 1) % arr.length;
  } else {
    flipIndex = (flipIndex - 1 + arr.length) % arr.length;
  }
  renderFlip3D();
}, { passive: false });

// 键盘快捷键支持 (Arrow / Tab / Enter / Escape)
window.addEventListener('keydown', (e) => {
  if (isFlip3D) {
    const arr = Array.from(activeTerms.values());
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown' || (e.key === 'Tab' && !e.shiftKey)) {
      e.preventDefault();
      flipIndex = (flipIndex + 1) % arr.length;
      renderFlip3D();
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp' || (e.key === 'Tab' && e.shiftKey)) {
      e.preventDefault();
      flipIndex = (flipIndex - 1 + arr.length) % arr.length;
      renderFlip3D();
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      exitFlip3D(true);
    } else if (e.key === 'Escape') {
      e.preventDefault();
      exitFlip3D(false);
    }
  } else {
    // 快捷键: Ctrl + Alt + T 新建终端
    if (e.ctrlKey && e.altKey && e.key.toLowerCase() === 't') {
      e.preventDefault();
      createTerminalWindow();
    }
    // 快捷键: Ctrl + Alt + F 开启 3D 层叠
    if (e.ctrlKey && e.altKey && e.key.toLowerCase() === 'f') {
      e.preventDefault();
      toggleFlip3D();
    }
  }
});

// -------------------------------------------------------------
// 6. 排列功能：网格平铺 (Tile) 与 经典阶梯排列 (Cascade)
// -------------------------------------------------------------

function tileWindows() {
  if (isFlip3D) exitFlip3D(false);
  const arr = Array.from(activeTerms.values());
  const count = arr.length;
  if (count === 0) return;

  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const topBarH = 64;
  const gap = 12;

  let cols = 1, rows = 1;
  if (count === 2) { cols = 2; rows = 1; }
  else if (count <= 4) { cols = 2; rows = 2; }
  else if (count <= 6) { cols = 3; rows = 2; }
  else if (count <= 9) { cols = 3; rows = 3; }
  else { cols = 4; rows = Math.ceil(count / 4); }

  const totalW = vw - gap * (cols + 1);
  const totalH = vh - topBarH - gap * (rows + 1);
  const w = Math.floor(totalW / cols);
  const h = Math.floor(totalH / rows);

  arr.forEach((item, idx) => {
    const c = idx % cols;
    const r = Math.floor(idx / cols);
    const left = gap + c * (w + gap);
    const top = topBarH + gap + r * (h + gap);

    item.dom.classList.add('snapped');
    item.dom.style.left = `${left}px`;
    item.dom.style.top = `${top}px`;
    item.dom.style.width = `${w}px`;
    item.dom.style.height = `${h}px`;
    item.isSnapped = true;
    item.snapMode = 'tile';

    setTimeout(() => {
      try {
        item.fit.fit();
        if (item.ws && item.ws.readyState === WebSocket.OPEN) {
          item.ws.send(JSON.stringify({ cols: item.term.cols, rows: item.term.rows }));
        }
      } catch (_) {}
    }, 200);
  });

  showToast(`已平铺排列 ${count} 个终端窗口`);
}

function cascadeWindows() {
  if (isFlip3D) exitFlip3D(false);
  const arr = Array.from(activeTerms.values());
  const count = arr.length;
  if (count === 0) return;

  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const initW = Math.min(Math.max(vw * 0.56, 520), 840);
  const initH = Math.min(Math.max(vh * 0.58, 320), 520);

  arr.forEach((item, idx) => {
    const left = Math.max(20 + idx * 32, 16);
    const top = Math.max(68 + idx * 26, 60);

    item.dom.classList.remove('snapped');
    item.dom.style.left = `${left}px`;
    item.dom.style.top = `${top}px`;
    item.dom.style.width = `${initW}px`;
    item.dom.style.height = `${initH}px`;
    highestZ++;
    item.dom.style.zIndex = highestZ;

    item.isSnapped = false;
    item.snapMode = null;
    item.origRect = { left, top, width: initW, height: initH };

    setTimeout(() => {
      try {
        item.fit.fit();
        if (item.ws && item.ws.readyState === WebSocket.OPEN) {
          item.ws.send(JSON.stringify({ cols: item.term.cols, rows: item.term.rows }));
        }
      } catch (_) {}
    }, 150);
  });

  showToast(`已阶梯排列 ${count} 个终端窗口`);
}

// -------------------------------------------------------------
// 7. 快捷记事本管理 (WinBox 挂载 + 可拖拽悬浮胶囊 + 键入标准输入)
// -------------------------------------------------------------

const STORAGE_KEY = 'attackbox_snippets_rows';
let snippets = [];

function initSnippets() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      snippets = JSON.parse(raw);
    } else {
      snippets = [
        'id; whoami; hostname',
        'ip addr show eth0',
        'uname -a',
        'ss -tulpn'
      ];
      saveSnippets();
    }
  } catch (e) {
    snippets = ['id; whoami; hostname'];
  }
}

function saveSnippets() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(snippets));
  } catch (e) {}
}

function renderSnippets() {
  const listEl = document.getElementById('np-list');
  if (!listEl) return;

  if (!snippets || snippets.length === 0) {
    listEl.innerHTML = `
      <div class="np-empty-state">
        <span>暂无常用命令或文本行</span>
        <span style="font-size: 11px; opacity: 0.7;">可在上方输入添加或粘贴批量导入</span>
      </div>`;
    return;
  }

  listEl.innerHTML = '';
  snippets.forEach((text, idx) => {
    const row = document.createElement('div');
    row.className = 'np-row';
    row.title = '单击复制到剪贴板，或点击「键入」发送至当前活动终端';

    const numSpan = document.createElement('span');
    numSpan.className = 'np-row-num';
    numSpan.textContent = idx + 1;

    const contentSpan = document.createElement('span');
    contentSpan.className = 'np-row-content';
    contentSpan.textContent = text;

    const actionsDiv = document.createElement('div');
    actionsDiv.className = 'np-row-actions';

    // 复制按钮
    const copyBtn = document.createElement('button');
    copyBtn.className = 'np-action-btn';
    copyBtn.textContent = '复制';
    copyBtn.title = '复制到系统剪贴板';
    copyBtn.onclick = (e) => {
      e.stopPropagation();
      copySnippet(text, row);
    };

    // 键入按钮 (直接输入到当前聚焦终端的标准输入中)
    const typeBtn = document.createElement('button');
    typeBtn.className = 'np-action-btn';
    typeBtn.textContent = '键入';
    typeBtn.title = '将该命令直接写入当前焦点终端';
    typeBtn.onclick = (e) => {
      e.stopPropagation();
      typeSnippetToTerminal(text);
    };

    // 删除按钮
    const delBtn = document.createElement('button');
    delBtn.className = 'np-action-btn np-btn-del';
    delBtn.textContent = '✕';
    delBtn.title = '删除此条';
    delBtn.onclick = (e) => {
      e.stopPropagation();
      snippets.splice(idx, 1);
      saveSnippets();
      renderSnippets();
    };

    actionsDiv.appendChild(copyBtn);
    actionsDiv.appendChild(typeBtn);
    actionsDiv.appendChild(delBtn);

    row.appendChild(numSpan);
    row.appendChild(contentSpan);
    row.appendChild(actionsDiv);

    // 单击整行直接复制
    row.onclick = () => {
      copySnippet(text, row);
    };

    listEl.appendChild(row);
  });
}

async function copySnippet(text, rowEl) {
  try {
    await navigator.clipboard.writeText(text);
    if (rowEl) {
      rowEl.classList.add('copied');
      setTimeout(() => rowEl.classList.remove('copied'), 1200);
    }
    showToast('已复制到系统剪贴板');
  } catch (err) {
    showToast('剪贴板复制受限');
  }
}

// 将文本发送至当前激活的终端标准输入
function typeSnippetToTerminal(text) {
  if (!text) return;
  let target = null;
  if (focusedWinId && activeTerms.has(focusedWinId)) {
    target = activeTerms.get(focusedWinId);
  } else if (activeTerms.size > 0) {
    target = Array.from(activeTerms.values())[0];
  }

  if (!target || !target.ws || target.ws.readyState !== WebSocket.OPEN) {
    showToast('暂无可输入的活动终端窗口');
    return;
  }

  // 发送数据至该 PTY
  const payload = text.endsWith('\n') ? text : text + '\n';
  target.ws.send(new TextEncoder().encode(payload));
  target.term.focus();
  showToast(`已向终端 #${target.num} 键入命令`);
}

let notesWinBox = null;
function toggleWinBox() {
  if (notesWinBox) {
    if (notesWinBox.min) {
      notesWinBox.minimize(false);
    } else {
      notesWinBox.close();
      notesWinBox = null;
    }
    return;
  }

  const mountEl = document.getElementById('notepad-app');
  mountEl.style.display = 'block';

  notesWinBox = new WinBox({
    title: "📋 快捷记事本 & 命令库",
    class: ["dark", "no-full"],
    width: "480px",
    height: "420px",
    minwidth: 320,
    minheight: 220,
    x: "right",
    y: "65px",
    top: 10,
    bottom: 10,
    left: 10,
    right: 16,
    mount: mountEl,
    onclose: () => {
      notesWinBox = null;
      document.querySelectorAll('.dock-btn').forEach(b => b.classList.remove('active'));
    }
  });

  document.querySelectorAll('.dock-btn').forEach(b => b.classList.add('active'));
  renderSnippets();
}

function setupNotepadEvents() {
  const inputEl = document.getElementById('np-input');
  const addBtn = document.getElementById('np-btn-add');
  const toggleBatchBtn = document.getElementById('np-btn-toggle-batch');
  const batchPanel = document.getElementById('np-batch-panel');
  const batchTextarea = document.getElementById('np-batch-textarea');
  const batchConfirmBtn = document.getElementById('np-btn-batch-confirm');
  const batchCancelBtn = document.getElementById('np-btn-batch-cancel');
  const clearAllBtn = document.getElementById('np-btn-clear-all');

  function addSingle() {
    const val = inputEl.value.trim();
    if (!val) return;
    snippets.unshift(val);
    inputEl.value = '';
    saveSnippets();
    renderSnippets();
  }

  addBtn.onclick = addSingle;
  inputEl.onkeydown = (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addSingle();
    }
  };

  toggleBatchBtn.onclick = () => {
    batchPanel.classList.toggle('hidden');
    if (!batchPanel.classList.contains('hidden')) {
      batchTextarea.focus();
    }
  };

  batchCancelBtn.onclick = () => {
    batchPanel.classList.add('hidden');
  };

  batchConfirmBtn.onclick = () => {
    const raw = batchTextarea.value;
    if (!raw.trim()) return;
    const lines = raw.split(/\r?\n/).map(l => l.trim()).filter(l => l.length > 0);
    if (lines.length > 0) {
      snippets.unshift(...lines);
      saveSnippets();
      renderSnippets();
      batchTextarea.value = '';
      batchPanel.classList.add('hidden');
      showToast(`已批量拆分导入 ${lines.length} 行`);
    }
  };

  clearAllBtn.onclick = () => {
    if (confirm('确认清空所有记事本行内容吗？')) {
      snippets = [];
      saveSnippets();
      renderSnippets();
      showToast('记事本已清空');
    }
  };
}

// -------------------------------------------------------------
// 8. 自由拖拽浮动 Dock 胶囊实现 (坐标记忆与点击去抖)
// -------------------------------------------------------------

function setupDraggableDock(dockEl, storageKey) {
  let isDragging = false;
  let startX = 0, startY = 0;
  let initLeft = 0, initTop = 0;
  let hasMoved = false;

  try {
    const saved = localStorage.getItem(storageKey);
    if (saved) {
      const pos = JSON.parse(saved);
      const maxL = window.innerWidth - (dockEl.offsetWidth || 110);
      const maxT = window.innerHeight - (dockEl.offsetHeight || 38);
      dockEl.style.left = Math.max(8, Math.min(pos.left, maxL - 8)) + 'px';
      dockEl.style.top = Math.max(8, Math.min(pos.top, maxT - 8)) + 'px';
      dockEl.style.right = 'auto';
    }
  } catch (e) {}

  dockEl.addEventListener('pointerdown', (e) => {
    if (e.button !== 0) return;
    isDragging = true;
    hasMoved = false;
    startX = e.clientX;
    startY = e.clientY;
    const rect = dockEl.getBoundingClientRect();
    initLeft = rect.left;
    initTop = rect.top;
    dockEl.setPointerCapture(e.pointerId);
    dockEl.classList.add('dragging');
  });

  dockEl.addEventListener('pointermove', (e) => {
    if (!isDragging) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) {
      hasMoved = true;
    }
    const maxL = window.innerWidth - dockEl.offsetWidth;
    const maxT = window.innerHeight - dockEl.offsetHeight;
    const nextLeft = Math.max(8, Math.min(initLeft + dx, maxL - 8));
    const nextTop = Math.max(8, Math.min(initTop + dy, maxT - 8));
    dockEl.style.left = nextLeft + 'px';
    dockEl.style.top = nextTop + 'px';
    dockEl.style.right = 'auto';
  });

  function stopDockDrag(e) {
    if (!isDragging) return;
    isDragging = false;
    try { dockEl.releasePointerCapture(e.pointerId); } catch (_) {}
    dockEl.classList.remove('dragging');
    if (hasMoved) {
      const rect = dockEl.getBoundingClientRect();
      try {
        localStorage.setItem(storageKey, JSON.stringify({ left: rect.left, top: rect.top }));
      } catch (_) {}
    }
  }

  dockEl.addEventListener('pointerup', stopDockDrag);
  dockEl.addEventListener('pointercancel', stopDockDrag);

  // 点击绑定 (拖动后不触发点击)
  dockEl.querySelector('.dock-btn').addEventListener('click', (e) => {
    if (hasMoved) return;
    toggleWinBox();
  });
}

// -------------------------------------------------------------
// 9. 页面入口初始化
// -------------------------------------------------------------

window.addEventListener('DOMContentLoaded', () => {
  initSnippets();
  setupNotepadEvents();

  const dockEl = document.getElementById('dock-notepad');
  if (dockEl) {
    setupDraggableDock(dockEl, 'attackbox_term_dock_pos');
  }

  // 默认启动一个主终端窗口
  createTerminalWindow();
});
