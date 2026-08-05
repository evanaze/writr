// ── State ──
let currentFile = null;
let currentDir = '.';
let wordGoal = 500;
let previewMode = false;
let splitMode = false;
let saveTimer = null;
let fileBaselineCount = 0;
let dailyWordCount = 0;   // persistent word count for today (from DB)
let lastFileWords = 0;    // word count of current file at last change
let wordCountTimer = null;

// ── DOM refs ──
const sidebar = document.getElementById('sidebar');
const tree = document.getElementById('tree');
const placeholder = document.getElementById('placeholder');
const editorWrapper = document.getElementById('editor-wrapper');
const editor = document.getElementById('editor');
const preview = document.getElementById('preview');
const currentFileEl = document.getElementById('current-file');
const wordCountEl = document.getElementById('word-count');
const goalDisplay = document.getElementById('goal-display');
const progressFill = document.getElementById('progress-fill');
const progressTrack = document.getElementById('progress-track');
const goalSetter = document.getElementById('goal-setter');
const goalInput = document.getElementById('goal-input');
const sidebarOpenBtn = document.getElementById('sidebar-open-btn');
const sidebarToggle = document.getElementById('sidebar-toggle');
const previewToggle = document.getElementById('preview-toggle');
const deleteFileBtn = document.getElementById('delete-file-btn');
const newFileBtn = document.getElementById('new-file-btn');

// Calendar widget refs
const streakCounter = document.getElementById('streak-counter');
const calTitle = document.getElementById('cal-title');
const calGrid = document.getElementById('calendar-grid');
const calPrev = document.getElementById('cal-prev');
const calNext = document.getElementById('cal-next');

// Calendar state (displayed month)
let calYear, calMonth;
let metDays = {};
let dayCounts = {};

// ── CodeMirror setup ──
const cm = CodeMirror.fromTextArea(editor, {
  mode: 'markdown',
  lineWrapping: true,
  autofocus: false,
});

// Start in vim mode by default
cm.setOption('keyMap', 'vim');

// Map :w / :wq to save (vim keymap calls cm.save())
cm.save = () => { saveFile(); };

// Insert markdown (e.g. for an uploaded image) at the cursor, or at the
// given document offset when a drop position is known.
function insertMarkdownAtCursor(markdown, at) {
  const cursor = at !== undefined ? cm.posFromIndex(at) : cm.getCursor();
  const before = cm.getRange({ line: 0, ch: 0 }, cursor);
  const insert = (before.length > 0 && !before.endsWith('\n') ? '\n' : '') + markdown + '\n';
  cm.replaceRange(insert, cursor);
  const end = cm.posFromIndex(cm.indexFromPos(cursor) + insert.length);
  cm.setCursor(end);
  cm.focus();
}


// ── Calendar ──
const DAY_HEADERS = ['S', 'M', 'T', 'W', 'T', 'F', 'S'];
const MONTH_NAMES = ['January','February','March','April','May','June','July','August','September','October','November','December'];

async function loadCalendar() {
  const now = new Date();
  if (calYear === undefined) {
    calYear = now.getFullYear();
    calMonth = now.getMonth() + 1; // 1-12
  }
  try {
    const data = await api('GET', `/api/calendar?year=${calYear}&month=${calMonth}`);
    metDays = data.days || {};
    dayCounts = data.counts || {};
    streakCounter.textContent = `🔥 ${data.streak}-day streak`;
    renderCalendar();
  } catch (e) {
    console.error('Failed to load calendar', e);
  }
}

function renderCalendar() {
  calTitle.textContent = MONTH_NAMES[calMonth - 1] + ' ' + calYear;
  calGrid.innerHTML = '';

  // Day-of-week headers (Sunday = 0)
  DAY_HEADERS.forEach(h => {
    const el = document.createElement('div');
    el.className = 'cal-day-head';
    el.textContent = h;
    calGrid.appendChild(el);
  });

  const first = new Date(calYear, calMonth - 1, 1);
  const daysInMonth = new Date(calYear, calMonth, 0).getDate();
  const startDow = first.getDay(); // 0=Sun
  const today = new Date();
  const todayStr = `${today.getFullYear()}-${String(today.getMonth()+1).padStart(2,'0')}-${String(today.getDate()).padStart(2,'0')}`;
  const thisMonth = today.getFullYear() === calYear && (today.getMonth()+1) === calMonth;

  // Leading empty cells
  for (let i = 0; i < startDow; i++) {
    const el = document.createElement('div');
    el.className = 'cal-day empty';
    calGrid.appendChild(el);
  }

  for (let d = 1; d <= daysInMonth; d++) {
    const el = document.createElement('div');
    el.className = 'cal-day';
    const dateStr = `${calYear}-${String(calMonth).padStart(2,'0')}-${String(d).padStart(2,'0')}`;
    el.textContent = d;
    if (metDays[dateStr]) {
      el.classList.add('met');
    }
    const n = dayCounts[dateStr] !== undefined ? dayCounts[dateStr] : 0;
    el.title = `${MONTH_NAMES[calMonth - 1]} ${d}, ${calYear}: ${n} ${n === 1 ? 'word' : 'words'}`;
    if (thisMonth && dateStr === todayStr) {
      el.classList.add('today');
    }
    calGrid.appendChild(el);
  }
}

function prevMonth() {
  calMonth--;
  if (calMonth < 1) { calMonth = 12; calYear--; }
  loadCalendar();
}

function nextMonth() {
  calMonth++;
  if (calMonth > 12) { calMonth = 1; calYear++; }
  loadCalendar();
}

calPrev.addEventListener('click', prevMonth);
calNext.addEventListener('click', nextMonth);

// ── Init ──
async function init() {
  const params = new URLSearchParams(window.location.search);
  const fileParam = params.get('file');
  const sidebarParam = params.get('sidebar');

  if (sidebarParam === 'hidden') {
    sidebar.classList.add('collapsed');
  }

  await loadGoal();
  await loadWordCount();
  await loadTree();
  await loadCalendar();

  if (fileParam) {
    openFile(fileParam);
  }
}

// ── API helpers ──
async function api(method, url, body) {
  const opts = { method };
  if (body) {
    if (body instanceof FormData) {
      opts.body = body;
    } else {
      opts.headers = { 'Content-Type': 'application/json' };
      opts.body = JSON.stringify(body);
    }
  }
  const res = await fetch(url, opts);
  if (!res.ok) {
    const err = await res.text();
    throw new Error(err || res.statusText);
  }
  const ct = res.headers.get('Content-Type') || '';
  if (ct.includes('application/json')) return res.json();
  return null;
}

// ── Goal ──
async function loadGoal() {
  try {
    const data = await api('GET', '/api/goal');
    wordGoal = parseInt(data.goal) || 500;
    goalDisplay.textContent = wordGoal;
    updateProgress();
  } catch (e) {
    console.error('Failed to load goal', e);
  }
}

async function saveGoal() {
  const v = parseInt(goalInput.value);
  if (!v || v < 1) return;
  wordGoal = v;
  goalDisplay.textContent = wordGoal;
  goalSetter.classList.add('hidden');
  await api('PUT', '/api/goal', { goal: String(v) });
  updateProgress();
}

// ── Tree ──
async function loadTree() {
  try {
    const nodes = await api('GET', '/api/tree');
    tree.innerHTML = '';
    renderTree(nodes, tree);
  } catch (e) {
    console.error('Failed to load tree', e);
  }
}

function renderTree(nodes, parent) {
  nodes.forEach(node => {
    const div = document.createElement('div');
    div.className = 'tree-item' + (node.isDir ? ' directory' : '');
    div.dataset.path = node.path;

    const icon = document.createElement('span');
    icon.className = 'icon';
    icon.textContent = node.isDir ? '▸' : '📄';
    div.appendChild(icon);

    const name = document.createElement('span');
    name.className = 'name';
    name.textContent = node.name;
    div.appendChild(name);

    div.addEventListener('click', (e) => {
      e.stopPropagation();
      if (node.isDir) {
        toggleDir(div, node);
      } else if (node.name.endsWith('.md')) {
        openFile(node.path);
      }
    });

    parent.appendChild(div);

    if (node.isDir && node.children) {
      const children = document.createElement('div');
      children.className = 'tree-children';
      children.style.display = 'none';
      renderTree(node.children, children);
      parent.appendChild(children);
    }
  });
}

function toggleDir(el, node) {
  const children = el.nextElementSibling;
  if (children && children.classList.contains('tree-children')) {
    const isOpen = children.style.display !== 'none';
    children.style.display = isOpen ? 'none' : 'block';
    el.querySelector('.icon').textContent = isOpen ? '▸' : '▾';
  }
}

// ── File operations ──
async function openFile(relPath) {
  try {
    const data = await api('GET', '/api/file?path=' + encodeURIComponent(relPath));
    currentFile = relPath;
    currentDir = relPath.includes('/') ? relPath.substring(0, relPath.lastIndexOf('/')) : '.';
    // Set baseline BEFORE setValue so the change event it fires computes a
    // delta of 0 (no spurious word-count change when switching files).
    fileBaselineCount = countWords(data.content);
    lastFileWords = fileBaselineCount;
    cm.setValue(data.content);
    placeholder.classList.add('hidden');
    editorWrapper.classList.remove('hidden');
    currentFileEl.textContent = relPath;
    deleteFileBtn.classList.remove('hidden');
    updateWordCount();
    updatePreview();
    highlightActiveFile();
    cm.refresh();
    cm.focus();
  } catch (e) {
    console.error('Failed to open file', e);
  }
}

function highlightActiveFile() {
  document.querySelectorAll('.tree-item').forEach(el => el.classList.remove('active'));
  const active = document.querySelector(`.tree-item[data-path="${currentFile}"]`);
  if (active) {
    active.classList.add('active');
    // Expand parent directories
    let parent = active.closest('.tree-children');
    while (parent) {
      parent.style.display = 'block';
      const prev = parent.previousElementSibling;
      if (prev && prev.classList.contains('tree-item')) {
        prev.querySelector('.icon').textContent = '▾';
      }
      parent = parent.parentElement.closest('.tree-children');
    }
  }
}

function saveFile() {
  if (!currentFile) return;
  api('PUT', '/api/file?path=' + encodeURIComponent(currentFile), { content: cm.getValue() })
    .then(() => loadCalendar())
    .catch(e => console.error('Failed to save', e));
}

async function deleteFile() {
  if (!currentFile) return;
  if (!confirm(`Delete "${currentFile}"?`)) return;
  try {
    await api('DELETE', '/api/file?path=' + encodeURIComponent(currentFile));
    currentFile = null;
    currentDir = '.';
    // Reset baseline before setValue to avoid spurious negative delta.
    fileBaselineCount = 0;
    lastFileWords = 0;
    cm.setValue('');
    placeholder.classList.remove('hidden');
    editorWrapper.classList.add('hidden');
    currentFileEl.textContent = '';
    deleteFileBtn.classList.add('hidden');
    updateWordCount();
    await loadTree();
  } catch (e) {
    console.error('Failed to delete', e);
  }
}

async function createFile() {
  const name = prompt('File name (e.g. notes.md):');
  if (!name) return;
  const fname = name.endsWith('.md') ? name : name + '.md';
  try {
    await api('POST', '/api/file?path=' + encodeURIComponent(fname));
    await loadTree();
    openFile(fname);
  } catch (e) {
    console.error('Failed to create file', e);
  }
}

// ── Editor ──
cm.on('change', () => {
  // Delta of words in the current file since the last change
  const currentWords = countWords(cm.getValue());
  const delta = currentWords - lastFileWords;
  lastFileWords = currentWords;
  if (delta !== 0) {
    dailyWordCount = Math.max(0, dailyWordCount + delta);
    updateWordCount();
    // Persist the delta to the DB (debounced)
    clearTimeout(wordCountTimer);
    wordCountTimer = setTimeout(() => {
      api('PUT', '/api/wordcount', { delta }).catch(e => console.error('Failed to persist word count', e));
    }, 300);
  }
  updatePreview();
  clearTimeout(saveTimer);
  saveTimer = setTimeout(saveFile, 500);
});

function countWords(text) {
  const t = text.trim();
  return t ? t.split(/\s+/).length : 0;
}

function stripFrontmatter(text) {
  const m = text.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/);
  return m ? text.slice(m[0].length) : text;
}

function newWordCount() {
  return dailyWordCount;
}

function updateWordCount() {
  const count = newWordCount();
  wordCountEl.textContent = count;
  updateProgress();
}

// Load the persistent daily word count from the database on startup so the
// counter survives app restarts.
async function loadWordCount() {
  try {
    const data = await api('GET', '/api/wordcount');
    dailyWordCount = data.count || 0;
    updateWordCount();
  } catch (e) {
    console.error('Failed to load word count', e);
  }
}

function updateProgress() {
  const count = newWordCount();
  const pct = Math.min(100, Math.round((count / wordGoal) * 100));
  progressFill.style.width = pct + '%';
  progressFill.className = 'progress-fill';
  if (count >= wordGoal && count < wordGoal * 1.1) progressFill.classList.add('complete');
  else if (count >= wordGoal * 1.1) progressFill.classList.add('over');
}

let mediumZoomInstance = null;

function updatePreview() {
  if (previewMode || splitMode) {
    preview.innerHTML = marked.parse(stripFrontmatter(cm.getValue())) || '';
    // Re-attach medium-zoom to preview images
    if (typeof mediumZoom !== 'undefined') {
      if (mediumZoomInstance) mediumZoomInstance.detach();
      const imgs = preview.querySelectorAll('img');
      if (imgs.length > 0) {
        mediumZoomInstance = mediumZoom(imgs, { background: 'rgba(0,0,0,0.85)' });
      }
    }
  }
}

// ── Preview toggle ──
previewToggle.addEventListener('click', () => {
  if (!splitMode && !previewMode) {
    // Off → split
    splitMode = true;
    previewMode = false;
    editorWrapper.classList.add('split');
    preview.classList.remove('hidden');
    previewToggle.textContent = '◑';
  } else if (splitMode) {
    // Split → preview only
    splitMode = false;
    previewMode = true;
    editorWrapper.classList.remove('split');
    cm.getWrapperElement().classList.add('hidden');
    preview.classList.remove('hidden');
    previewToggle.textContent = '✎';
  } else {
    // Preview only → off
    previewMode = false;
    cm.getWrapperElement().classList.remove('hidden');
    cm.refresh();
    preview.classList.add('hidden');
    editorWrapper.classList.remove('split');
    previewToggle.textContent = '◐';
  }
  updatePreview();
});

// ── Progress bar click ──
progressTrack.addEventListener('click', () => {
  goalSetter.classList.toggle('hidden');
  if (!goalSetter.classList.contains('hidden')) {
    goalInput.value = wordGoal;
    goalInput.focus();
  }
});

document.getElementById('goal-save').addEventListener('click', saveGoal);
goalInput.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') saveGoal();
  if (e.key === 'Escape') goalSetter.classList.add('hidden');
});

// ── Sidebar toggle ──
sidebarToggle.addEventListener('click', () => {
  sidebar.classList.add('collapsed');
});
sidebarOpenBtn.addEventListener('click', () => {
  sidebar.classList.remove('collapsed');
});

// ── Delete ──
deleteFileBtn.addEventListener('click', deleteFile);
newFileBtn.addEventListener('click', createFile);

// ── Drag & drop images ──
const cmWrapper = cm.getWrapperElement();
cmWrapper.addEventListener('dragover', (e) => {
  e.preventDefault();
  cmWrapper.classList.add('drag-over');
});
cmWrapper.addEventListener('dragleave', () => {
  cmWrapper.classList.remove('drag-over');
});
cmWrapper.addEventListener('drop', async (e) => {
  e.preventDefault();
  cmWrapper.classList.remove('drag-over');
  const files = e.dataTransfer.files;
  if (!files.length) return;
  const dropPos = cm.coordsChar({ left: e.clientX, top: e.clientY });
  const at = cm.indexFromPos(dropPos);
  for (const file of files) {
    if (!file.type.startsWith('image/')) continue;
    try {
      const form = new FormData();
      form.append('file', file);
      const data = await api('POST', '/api/upload?dir=' + encodeURIComponent(currentDir), form);
      insertMarkdownAtCursor(data.markdown, at);
      updateWordCount();
      updatePreview();
      saveFile();
      loadTree();
    } catch (e) {
      console.error('Upload failed', e);
    }
  }
});

// ── Clipboard paste images ──
editor.addEventListener('paste', async (e) => {
  const items = e.clipboardData?.items;
  if (!items) return;
  for (const item of items) {
    if (!item.type.startsWith('image/')) continue;
    e.preventDefault();
    const file = item.getAsFile();
    if (!file) continue;
    try {
      const form = new FormData();
      // Generate a reasonable filename with correct extension
      const ext = item.type.split('/')[1] || 'png';
      form.append('file', file, 'pasted-' + Date.now() + '.' + ext);
      const data = await api('POST', '/api/upload?dir=' + encodeURIComponent(currentDir), form);
      insertMarkdownAtCursor(data.markdown);
      updateWordCount();
      updatePreview();
      saveFile();
      loadTree();
    } catch (err) {
      console.error('Paste upload failed', err);
    }
    break;
  }
});

// ── Keyboard shortcuts ──
cm.setOption('extraKeys', {
  'Cmd-S': saveFile,
  'Ctrl-S': saveFile,
});
document.addEventListener('keydown', (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault();
    saveFile();
  }
});

// ── Start ──
init();
