// =============================================================================
// myc Desktop Application Logic - Total Commander Frontend
// =============================================================================

const state = {
  activePanel: 'left',
  drives: [],
  left: {
    path: '',
    items: [],
    selected: new Set(),
    cursor: 0,
    sortCol: 'name',
    sortAsc: true,
    totalSize: 0,
    freeSize: 0,
    tabs: [],
    activeTab: 0
  },
  right: {
    path: '',
    items: [],
    selected: new Set(),
    cursor: 0,
    sortCol: 'name',
    sortAsc: true,
    totalSize: 0,
    freeSize: 0,
    tabs: [],
    activeTab: 0
  },
  currentListerFile: null,
  listerRawContent: '',
  listerHexContent: '',
  listerIsHex: false,
  currentEditorFile: null
};

// --- Initialization ---
window.addEventListener('DOMContentLoaded', async () => {
  // Start heartbeat
  setInterval(sendHeartbeat, 2500);

  // Fetch initial info & drives
  try {
    const infoRes = await fetch('/api/info');
    const info = await infoRes.json();

    const drivesRes = await fetch('/api/drives');
    state.drives = await drivesRes.json();
    renderDrives('left');
    renderDrives('right');

    const leftInit = info.initial_left || '.';
    const rightInit = info.initial_right || '..';

    // Setup initial tabs
    state.left.tabs = [{ path: leftInit, name: getTabName(leftInit) }];
    state.right.tabs = [{ path: rightInit, name: getTabName(rightInit) }];

    await Promise.all([
      loadPanel('left', leftInit),
      loadPanel('right', rightInit)
    ]);

    setActivePanel('left');
  } catch (err) {
    console.error('Błąd inicjalizacji myc:', err);
  }

  setupGlobalKeys();
});

function getTabName(p) {
  if (!p || p === '/' || p.endsWith(':\\')) return p;
  const parts = p.replace(/\\/g, '/').split('/').filter(Boolean);
  return parts.length ? parts[parts.length - 1] : p;
}

function sendHeartbeat() {
  fetch('/api/heartbeat', { method: 'POST' }).catch(() => {});
}

// --- Drives Rendering ---
function renderDrives(panelId) {
  const container = document.getElementById(`${panelId}-drive-btns`);
  container.innerHTML = '';
  state.drives.forEach(d => {
    const btn = document.createElement('button');
    btn.className = 'drive-btn';
    btn.textContent = `[-${d.letter.toLowerCase()}-]`;
    btn.title = `${d.name} (${formatSize(d.freeSize)} wolne)`;
    btn.onclick = () => {
      loadPanel(panelId, d.path);
    };
    container.appendChild(btn);
  });
}

// --- Tabs Management ---
function renderTabs(panelId) {
  const p = state[panelId];
  const list = document.getElementById(`${panelId}-tabs-list`);
  list.innerHTML = '';
  p.tabs.forEach((tab, idx) => {
    const tabEl = document.createElement('div');
    tabEl.className = `panel-tab ${idx === p.activeTab ? 'active' : ''}`;
    tabEl.innerHTML = `
      <span class="tab-title">${escapeHTML(tab.name)}</span>
      ${p.tabs.length > 1 ? `<span class="tab-close" onclick="closeTab('${panelId}', ${idx}, event)">×</span>` : ''}
    `;
    tabEl.onclick = () => switchTab(panelId, idx);
    list.appendChild(tabEl);
  });
}

function addTab(panelId) {
  const p = state[panelId];
  p.tabs.push({ path: p.path, name: getTabName(p.path) });
  p.activeTab = p.tabs.length - 1;
  renderTabs(panelId);
}

function closeTab(panelId, idx, e) {
  if (e) e.stopPropagation();
  const p = state[panelId];
  if (p.tabs.length <= 1) return;
  p.tabs.splice(idx, 1);
  if (p.activeTab >= p.tabs.length) p.activeTab = p.tabs.length - 1;
  renderTabs(panelId);
  loadPanel(panelId, p.tabs[p.activeTab].path);
}

function switchTab(panelId, idx) {
  const p = state[panelId];
  p.activeTab = idx;
  renderTabs(panelId);
  loadPanel(panelId, p.tabs[idx].path);
}

// --- Panel Navigation & Data Loading ---
async function loadPanel(panelId, targetPath) {
  const p = state[panelId];
  const queryPath = targetPath !== undefined ? targetPath : p.path;

  try {
    const res = await fetch(`/api/list?path=${encodeURIComponent(queryPath)}`);
    if (!res.ok) {
      const errData = await res.json();
      alert(`Błąd: ${errData.error || 'Nie można załadować katalogu'}`);
      return;
    }
    const data = await res.json();
    p.path = data.path;
    p.items = data.items || [];
    p.selected.clear();
    p.cursor = 0;
    p.totalSize = data.total_size;
    p.freeSize = data.free_size;

    // Update active tab path
    if (p.tabs[p.activeTab]) {
      p.tabs[p.activeTab].path = p.path;
      p.tabs[p.activeTab].name = getTabName(p.path);
    }

    document.getElementById(`${panelId}-path-input`).value = p.path;
    document.getElementById(`${panelId}-drive-space`).textContent =
      data.free_size ? `${formatSize(data.free_size)} wolne z ${formatSize(data.total_size)}` : '';

    if (panelId === state.activePanel) {
      updateCmdPrompt();
    }

    sortItems(panelId);
    renderTabs(panelId);
    renderTable(panelId);
    updateStatus(panelId);
  } catch (err) {
    console.error(`Błąd wczytywania panelu ${panelId}:`, err);
  }
}

function navigatePanel(panelId, newPath) {
  loadPanel(panelId, newPath);
}

function goParent(panelId) {
  const p = state[panelId];
  if (p.path.includes('::/')) {
    const parts = p.path.split('::/');
    const archivePath = parts[0];
    const sub = parts[1];
    if (!sub || sub === '/') {
      loadPanel(panelId, archivePath.substring(0, archivePath.lastIndexOf('/')) || '/');
      return;
    }
    const subParts = sub.split('/').filter(Boolean);
    subParts.pop();
    const newSub = subParts.join('/');
    loadPanel(panelId, archivePath + (newSub ? '::/' + newSub : ''));
    return;
  }
  const parent = p.path.replace(/\\/g, '/').split('/').filter(Boolean);
  if (parent.length > 0) parent.pop();
  let parentPath = parent.join('/');
  if (!parentPath.includes('/') && !parentPath.includes(':')) parentPath = '/' + parentPath;
  if (parentPath === '') parentPath = '/';
  loadPanel(panelId, parentPath);
}

function goHome(panelId) {
  const homeDrive = state.drives.find(d => d.letter === '~');
  if (homeDrive) loadPanel(panelId, homeDrive.path);
  else loadPanel(panelId, '/');
}

// --- Sorting ---
function sortPanel(panelId, col) {
  const p = state[panelId];
  if (p.sortCol === col) {
    p.sortAsc = !p.sortAsc;
  } else {
    p.sortCol = col;
    p.sortAsc = true;
  }
  sortItems(panelId);
  renderTable(panelId);
}

function sortItems(panelId) {
  const p = state[panelId];
  const col = p.sortCol;
  const asc = p.sortAsc;

  p.items.sort((a, b) => {
    // Parent .. always on top
    if (a.name === '..') return -1;
    if (b.name === '..') return 1;

    // Directories first
    if (a.is_dir && !b.is_dir) return -1;
    if (!a.is_dir && b.is_dir) return 1;

    let res = 0;
    if (col === 'name') {
      res = a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    } else if (col === 'ext') {
      res = (a.ext || '').localeCompare(b.ext || '', undefined, { sensitivity: 'base' });
    } else if (col === 'size') {
      res = a.size - b.size;
    } else if (col === 'date') {
      res = a.mod_time.localeCompare(b.mod_time);
    }
    return asc ? res : -res;
  });
}

// --- Table Rendering ---
function renderTable(panelId) {
  const p = state[panelId];
  const tbody = document.getElementById(`${panelId}-file-tbody`);
  tbody.innerHTML = '';

  p.items.forEach((item, idx) => {
    const tr = document.createElement('tr');
    tr.className = `file-row ${idx === p.cursor ? 'cursor' : ''} ${p.selected.has(idx) ? 'selected' : ''}`;
    tr.onclick = (e) => {
      p.cursor = idx;
      if (e.ctrlKey) {
        toggleSelect(panelId, idx);
      }
      renderTable(panelId);
      updateStatus(panelId);
    };
    tr.ondblclick = () => openItem(panelId, idx);

    let icon = '📄';
    if (item.name === '..') icon = '📁';
    else if (item.is_dir) icon = '📁';
    else if (item.is_archive) icon = '🗜️';

    let sizeStr = '';
    if (item.name === '..') sizeStr = '&lt;DIR&gt;';
    else if (item.is_dir) sizeStr = '&lt;DIR&gt;';
    else if (item.is_archive) sizeStr = '&lt;ARCH&gt;';
    else sizeStr = formatBytes(item.size);

    tr.innerHTML = `
      <td class="col-name"><span class="icon-badge">${icon}</span>${escapeHTML(item.name)}</td>
      <td class="col-ext">${escapeHTML(item.ext || '')}</td>
      <td class="col-size">${sizeStr}</td>
      <td class="col-date">${item.mod_time || ''}</td>
      <td class="col-attr">${item.mode || ''}</td>
    `;
    tbody.appendChild(tr);
  });

  scrollCursorIntoView(panelId);
}

function scrollCursorIntoView(panelId) {
  const scrollBox = document.getElementById(`${panelId}-table-scroll`);
  const tbody = document.getElementById(`${panelId}-file-tbody`);
  const rows = tbody.querySelectorAll('.file-row');
  const p = state[panelId];
  if (rows[p.cursor]) {
    const row = rows[p.cursor];
    const top = row.offsetTop;
    const bottom = top + row.offsetHeight;
    if (top < scrollBox.scrollTop) {
      scrollBox.scrollTop = top;
    } else if (bottom > scrollBox.scrollTop + scrollBox.clientHeight) {
      scrollBox.scrollTop = bottom - scrollBox.clientHeight;
    }
  }
}

function updateStatus(panelId) {
  const p = state[panelId];
  const totalCount = p.items.filter(i => i.name !== '..').length;
  let totalBytes = 0;
  let selBytes = 0;
  let selCount = 0;

  p.items.forEach((item, idx) => {
    if (item.name !== '..' && !item.is_dir) {
      totalBytes += item.size;
    }
    if (p.selected.has(idx)) {
      selCount++;
      if (!item.is_dir) selBytes += item.size;
    }
  });

  const text = `${formatBytes(selBytes)} / ${formatBytes(totalBytes)} w ${selCount} / ${totalCount} plikach`;
  document.getElementById(`${panelId}-status-text`).textContent = text;
}

// --- Active Panel ---
function setActivePanel(panelId) {
  state.activePanel = panelId;
  document.getElementById('left-panel').classList.toggle('active-panel', panelId === 'left');
  document.getElementById('right-panel').classList.toggle('active-panel', panelId === 'right');
  updateCmdPrompt();
}

function updateCmdPrompt() {
  const p = state[state.activePanel];
  document.getElementById('cmd-prompt').textContent = (p.path ? p.path + '>' : '> ');
}

// --- Selection Functions (Total Commander Rules) ---
function toggleSelect(panelId, idx) {
  const p = state[panelId];
  if (p.items[idx] && p.items[idx].name === '..') return;
  if (p.selected.has(idx)) p.selected.delete(idx);
  else p.selected.add(idx);
}

function toggleSelectAndAdvance(panelId) {
  const p = state[panelId];
  toggleSelect(panelId, p.cursor);
  if (p.cursor < p.items.length - 1) {
    p.cursor++;
  }
  renderTable(panelId);
  updateStatus(panelId);
}

function selectAll() {
  const p = state[state.activePanel];
  p.items.forEach((item, idx) => {
    if (item.name !== '..') p.selected.add(idx);
  });
  renderTable(state.activePanel);
  updateStatus(state.activePanel);
}

function invertSelection() {
  const p = state[state.activePanel];
  p.items.forEach((item, idx) => {
    if (item.name !== '..') {
      if (p.selected.has(idx)) p.selected.delete(idx);
      else p.selected.add(idx);
    }
  });
  renderTable(state.activePanel);
  updateStatus(state.activePanel);
}

function selectPatternPrompt() {
  const pattern = prompt('Zaznacz pliki według wzorca (np. *.go, *test*):', '*.*');
  if (!pattern) return;
  const p = state[state.activePanel];
  const regex = globToRegex(pattern);
  p.items.forEach((item, idx) => {
    if (item.name !== '..' && regex.test(item.name)) {
      p.selected.add(idx);
    }
  });
  renderTable(state.activePanel);
  updateStatus(state.activePanel);
}

function unselectPatternPrompt() {
  const pattern = prompt('Odznacz pliki według wzorca:', '*.*');
  if (!pattern) return;
  const p = state[state.activePanel];
  const regex = globToRegex(pattern);
  p.items.forEach((item, idx) => {
    if (item.name !== '..' && regex.test(item.name)) {
      p.selected.delete(idx);
    }
  });
  renderTable(state.activePanel);
  updateStatus(state.activePanel);
}

function globToRegex(glob) {
  const escaped = glob.replace(/[.+^${}()|[\]\\]/g, '\\$&');
  const re = escaped.replace(/\*/g, '.*').replace(/\?/g, '.');
  return new RegExp('^' + re + '$', 'i');
}

// --- Opening Items (Directories, Archives, Files) ---
function openItem(panelId, idx) {
  const p = state[panelId];
  const item = p.items[idx];
  if (!item) return;

  if (item.name === '..') {
    goParent(panelId);
    return;
  }

  if (item.is_dir || item.is_archive) {
    loadPanel(panelId, item.path);
    return;
  }

  // Regular file: open in Lister
  viewFile(item.path);
}

// --- Keyboard Navigation Handler ---
function setupGlobalKeys() {
  window.addEventListener('keydown', (e) => {
    // If inside an open modal, allow Esc to close modal
    const openModal = document.querySelector('.modal-dialog[style*="display: flex"]');
    if (openModal) {
      if (e.key === 'Escape') {
        closeModals();
      }
      return;
    }

    // If focused inside an input field, do not hijack arrows/enter
    if (['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement.tagName)) {
      return;
    }

    const panelId = state.activePanel;
    const p = state[panelId];

    if (e.key === 'Tab') {
      e.preventDefault();
      setActivePanel(panelId === 'left' ? 'right' : 'left');
      return;
    }

    if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (p.cursor > 0) {
        p.cursor--;
        renderTable(panelId);
      }
      return;
    }

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (p.cursor < p.items.length - 1) {
        p.cursor++;
        renderTable(panelId);
      }
      return;
    }

    if (e.key === 'Home') {
      e.preventDefault();
      p.cursor = 0;
      renderTable(panelId);
      return;
    }

    if (e.key === 'End') {
      e.preventDefault();
      p.cursor = Math.max(0, p.items.length - 1);
      renderTable(panelId);
      return;
    }

    if (e.key === 'PageUp') {
      e.preventDefault();
      p.cursor = Math.max(0, p.cursor - 15);
      renderTable(panelId);
      return;
    }

    if (e.key === 'PageDown') {
      e.preventDefault();
      p.cursor = Math.min(p.items.length - 1, p.cursor + 15);
      renderTable(panelId);
      return;
    }

    if (e.key === 'Insert' || e.code === 'Space') {
      e.preventDefault();
      toggleSelectAndAdvance(panelId);
      return;
    }

    if (e.key === 'Enter') {
      e.preventDefault();
      openItem(panelId, p.cursor);
      return;
    }

    if (e.key === 'Backspace') {
      e.preventDefault();
      goParent(panelId);
      return;
    }

    if (e.key === '+') {
      e.preventDefault();
      selectPatternPrompt();
      return;
    }

    if (e.key === '-') {
      e.preventDefault();
      unselectPatternPrompt();
      return;
    }

    if (e.key === '*') {
      e.preventDefault();
      invertSelection();
      return;
    }

    if (e.ctrlKey && (e.key === 'a' || e.key === 'A')) {
      e.preventDefault();
      selectAll();
      return;
    }

    if (e.ctrlKey && (e.key === 'r' || e.key === 'R')) {
      e.preventDefault();
      refreshBothPanels();
      return;
    }

    if (e.ctrlKey && (e.key === 'f' || e.key === 'F')) {
      e.preventDefault();
      showSearchDialog();
      return;
    }

    if (e.ctrlKey && (e.key === 'm' || e.key === 'M')) {
      e.preventDefault();
      showMultiRenameDialog();
      return;
    }

    // Function keys F1..F10
    if (e.key === 'F1') { e.preventDefault(); showHelp(); }
    else if (e.key === 'F2') { e.preventDefault(); refreshBothPanels(); }
    else if (e.key === 'F3') { e.preventDefault(); showLister(); }
    else if (e.key === 'F4') { e.preventDefault(); showEditor(); }
    else if (e.key === 'F5') { e.preventDefault(); showCopyDialog(); }
    else if (e.key === 'F6') { e.preventDefault(); showMoveDialog(); }
    else if (e.key === 'F7') { e.preventDefault(); showMkdirDialog(); }
    else if (e.key === 'F8') { e.preventDefault(); showDeleteDialog(); }
    else if (e.altKey && e.key === 'F7') { e.preventDefault(); showSearchDialog(); }
    else if (e.altKey && e.key === 'F4') { e.preventDefault(); exitApp(); }
  });
}

// --- Helpers to get Selected or Cursor Files ---
function getActiveSelection() {
  const p = state[state.activePanel];
  const items = [];
  if (p.selected.size > 0) {
    p.selected.forEach(idx => {
      const it = p.items[idx];
      if (it && it.name !== '..') items.push(it);
    });
  } else if (p.items[p.cursor] && p.items[p.cursor].name !== '..') {
    items.push(p.items[p.cursor]);
  }
  return items;
}

function getInactivePanelPath() {
  const otherId = state.activePanel === 'left' ? 'right' : 'left';
  return state[otherId].path;
}

// --- Lister (F3) ---
async function showLister() {
  const items = getActiveSelection();
  if (items.length === 0) return;
  viewFile(items[0].path);
}

async function viewFile(path) {
  try {
    const res = await fetch(`/api/file?path=${encodeURIComponent(path)}`);
    if (!res.ok) {
      alert('Nie można odczytać pliku');
      return;
    }
    const data = await res.json();
    state.currentListerFile = path;
    state.listerRawContent = data.content || '';
    state.listerHexContent = data.hex || '';
    state.listerIsHex = data.is_binary;

    document.getElementById('lister-title').textContent = `Podgląd: ${path}`;
    document.getElementById('lister-info').textContent = `Rozmiar: ${formatBytes(data.size)} | ${data.is_binary ? 'Plik binarny' : 'Plik tekstowy'}`;
    document.getElementById('lister-content').textContent = state.listerIsHex ? state.listerHexContent : state.listerRawContent;

    openModal('modal-lister');
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

function toggleListerHex() {
  state.listerIsHex = !state.listerIsHex;
  document.getElementById('lister-content').textContent = state.listerIsHex ? state.listerHexContent : state.listerRawContent;
}

// --- Editor (F4) ---
async function showEditor() {
  const items = getActiveSelection();
  if (items.length === 0) return;
  const path = items[0].path;

  try {
    const res = await fetch(`/api/file?path=${encodeURIComponent(path)}`);
    if (!res.ok) {
      alert('Nie można otworzyć pliku do edycji');
      return;
    }
    const data = await res.json();
    if (data.is_binary) {
      if (!confirm('Plik wydaje się być binarny. Czy na pewno chcesz go edytować jako tekst?')) {
        return;
      }
    }
    state.currentEditorFile = path;
    document.getElementById('editor-title').textContent = `Edycja pliku: ${path}`;
    document.getElementById('editor-textarea').value = data.content || '';
    document.getElementById('editor-status').textContent = `Rozmiar: ${formatBytes(data.size)}`;
    openModal('modal-editor');
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

async function saveEditorContent() {
  if (!state.currentEditorFile) return;
  const content = document.getElementById('editor-textarea').value;
  try {
    const res = await fetch('/api/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: state.currentEditorFile, content })
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Błąd zapisu: ${err.error}`);
      return;
    }
    document.getElementById('editor-status').textContent = 'Zapisano pomyślnie!';
    setTimeout(() => closeModal('modal-editor'), 400);
    refreshBothPanels();
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

// --- Copy (F5) ---
function showCopyDialog() {
  const items = getActiveSelection();
  if (items.length === 0) return;
  document.getElementById('copy-summary').textContent = `Kopiuj ${items.length} plik(ów) do:`;
  document.getElementById('copy-dest-input').value = getInactivePanelPath();
  openModal('modal-copy');
}

async function executeCopy() {
  const items = getActiveSelection();
  const dest = document.getElementById('copy-dest-input').value;
  if (!dest) return;
  closeModal('modal-copy');

  try {
    const res = await fetch('/api/copy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ sources: items.map(i => i.path), destination: dest })
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Błąd kopiowania: ${err.error}`);
      return;
    }
    refreshBothPanels();
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

// --- Move / Rename (F6) ---
function showMoveDialog() {
  const items = getActiveSelection();
  if (items.length === 0) return;
  if (items.length === 1) {
    document.getElementById('move-summary').textContent = `Zmień nazwę / przenieś do:`;
    document.getElementById('move-dest-input').value = getInactivePanelPath();
  } else {
    document.getElementById('move-summary').textContent = `Przenieś ${items.length} plik(ów) do:`;
    document.getElementById('move-dest-input').value = getInactivePanelPath();
  }
  openModal('modal-move');
}

async function executeMove() {
  const items = getActiveSelection();
  const dest = document.getElementById('move-dest-input').value;
  if (!dest) return;
  closeModal('modal-move');

  try {
    const res = await fetch('/api/move', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ sources: items.map(i => i.path), destination: dest })
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Błąd przenoszenia: ${err.error}`);
      return;
    }
    refreshBothPanels();
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

// --- Mkdir (F7) ---
function showMkdirDialog() {
  document.getElementById('mkdir-name-input').value = 'Nowy katalog';
  openModal('modal-mkdir');
  setTimeout(() => {
    const input = document.getElementById('mkdir-name-input');
    input.focus();
    input.select();
  }, 50);
}

async function executeMkdir() {
  const name = document.getElementById('mkdir-name-input').value.trim();
  if (!name) return;
  const p = state[state.activePanel];
  closeModal('modal-mkdir');

  try {
    const res = await fetch('/api/mkdir', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: p.path, name })
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Błąd: ${err.error}`);
      return;
    }
    refreshActivePanel();
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

// --- Delete (F8) ---
function showDeleteDialog() {
  const items = getActiveSelection();
  if (items.length === 0) return;
  const list = document.getElementById('delete-list');
  list.innerHTML = '';
  items.forEach(i => {
    const li = document.createElement('li');
    li.textContent = i.name;
    list.appendChild(li);
  });
  document.getElementById('delete-summary').textContent = `Czy na pewno chcesz usunąć ${items.length} element(ów)?`;
  openModal('modal-delete');
}

async function executeDelete() {
  const items = getActiveSelection();
  closeModal('modal-delete');

  try {
    const res = await fetch('/api/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths: items.map(i => i.path) })
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Błąd usuwania: ${err.error}`);
      return;
    }
    refreshBothPanels();
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

// --- Search Dialog (Ctrl+F) ---
function showSearchDialog() {
  const p = state[state.activePanel];
  document.getElementById('search-root').value = p.path;
  document.getElementById('search-results-tbody').innerHTML = '';
  openModal('modal-search');
}

async function executeSearch() {
  const root = document.getElementById('search-root').value;
  const pattern = document.getElementById('search-pattern').value;
  const content = document.getElementById('search-content').value;
  const caseSensitive = document.getElementById('search-case').checked;
  const tbody = document.getElementById('search-results-tbody');
  tbody.innerHTML = '<tr><td colspan="3" style="text-align:center; padding:10px;">Szukanie w toku...</td></tr>';

  try {
    const res = await fetch('/api/search', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ root_dir: root, pattern, content, case_sensitive: caseSensitive })
    });
    const matches = await res.json();
    tbody.innerHTML = '';
    if (matches.length === 0) {
      tbody.innerHTML = '<tr><td colspan="3" style="text-align:center; padding:10px;">Nie znaleziono pasujących plików.</td></tr>';
      return;
    }
    matches.forEach(m => {
      const tr = document.createElement('tr');
      tr.className = 'file-row';
      tr.innerHTML = `
        <td style="cursor:pointer;" title="${escapeHTML(m.Path)}">${escapeHTML(m.Path)}</td>
        <td>${formatBytes(m.Size)}</td>
        <td>${m.LineNumber ? `Linia ${m.LineNumber}: ${escapeHTML(m.LineText.substring(0, 50))}` : '-'}</td>
      `;
      tr.ondblclick = () => {
        closeModal('modal-search');
        viewFile(m.Path);
      };
      tbody.appendChild(tr);
    });
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="3" style="color:red; text-align:center;">Błąd: ${err.message}</td></tr>`;
  }
}

// --- Multi-Rename (Ctrl+M) ---
let currentMRPairs = [];
function showMultiRenameDialog() {
  const items = getActiveSelection();
  if (items.length === 0) {
    alert('Zaznacz pliki, którym chcesz zmienić nazwy.');
    return;
  }
  openModal('modal-multirename');
  previewMultiRename();
}

async function previewMultiRename() {
  const items = getActiveSelection();
  const prefix = document.getElementById('mr-prefix').value;
  const suffix = document.getElementById('mr-suffix').value;
  const find = document.getElementById('mr-find').value;
  const replace = document.getElementById('mr-replace').value;
  const startNum = parseInt(document.getElementById('mr-start').value, 10) || 1;
  const digits = parseInt(document.getElementById('mr-digits').value, 10) || 3;

  try {
    const res = await fetch('/api/multi-rename/preview', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        files: items.map(i => i.path),
        prefix, suffix, find, replace,
        start_num: startNum, digits
      })
    });
    currentMRPairs = await res.json();
    const tbody = document.getElementById('mr-preview-tbody');
    tbody.innerHTML = '';
    currentMRPairs.forEach(p => {
      const tr = document.createElement('tr');
      tr.innerHTML = `<td>${escapeHTML(p.OldName)}</td><td style="color:#008000; font-weight:bold;">${escapeHTML(p.NewName)}</td>`;
      tbody.appendChild(tr);
    });
  } catch (err) {
    console.error('Błąd podglądu multi-rename:', err);
  }
}

async function applyMultiRename() {
  if (!currentMRPairs.length) return;
  closeModal('modal-multirename');
  try {
    const res = await fetch('/api/multi-rename/apply', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(currentMRPairs)
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Błąd: ${err.error}`);
      return;
    }
    refreshBothPanels();
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

// --- Synchronize Directories ---
function showSyncDialog() {
  document.getElementById('sync-left-path').textContent = state.left.path;
  document.getElementById('sync-right-path').textContent = state.right.path;
  document.getElementById('sync-results-tbody').innerHTML = '';
  openModal('modal-sync');
}

async function executeCompareSync() {
  const tbody = document.getElementById('sync-results-tbody');
  tbody.innerHTML = '<tr><td colspan="5" style="text-align:center; padding:10px;">Porównywanie katalogów...</td></tr>';
  try {
    const res = await fetch('/api/sync', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ left_dir: state.left.path, right_dir: state.right.path })
    });
    const items = await res.json();
    tbody.innerHTML = '';
    if (!items || items.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" style="text-align:center; padding:10px;">Katalogi są identyczne.</td></tr>';
      return;
    }
    items.forEach(it => {
      const tr = document.createElement('tr');
      let arrow = '==';
      let color = '#555';
      if (it.Status === 'LEFT_NEWER' || it.Status === 'LEFT_ONLY') { arrow = '-->'; color = '#008000'; }
      else if (it.Status === 'RIGHT_NEWER' || it.Status === 'RIGHT_ONLY') { arrow = '<--'; color = '#0000cc'; }
      else if (it.Status === 'SIZE_DIFF') { arrow = '<!>'; color = '#cc0000'; }

      tr.innerHTML = `
        <td>${escapeHTML(it.RelPath)}</td>
        <td>${it.LeftSize !== -1 ? formatBytes(it.LeftSize) : '-'}</td>
        <td style="font-weight:bold; color:${color}; text-align:center;">${arrow}</td>
        <td>${it.RightSize !== -1 ? formatBytes(it.RightSize) : '-'}</td>
        <td>${it.Status}</td>
      `;
      tbody.appendChild(tr);
    });
  } catch (err) {
    tbody.innerHTML = `<tr><td colspan="5" style="color:red;">Błąd: ${err.message}</td></tr>`;
  }
}

// --- Split & Join ---
function showSplitDialog() {
  const items = getActiveSelection();
  if (items.length === 0 || items[0].is_dir) {
    alert('Wybierz plik do podziału.');
    return;
  }
  document.getElementById('split-file-name').textContent = `Plik: ${items[0].name}`;
  openModal('modal-split');
}

async function executeSplit() {
  const items = getActiveSelection();
  const chunkSize = parseInt(document.getElementById('split-size-select').value, 10);
  const dstDir = getInactivePanelPath();
  closeModal('modal-split');

  try {
    const res = await fetch('/api/split', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ file: items[0].path, dst_dir: dstDir, chunk_size: chunkSize })
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Błąd: ${err.error}`);
      return;
    }
    refreshBothPanels();
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

function showJoinDialog() {
  const items = getActiveSelection();
  if (items.length === 0) {
    alert('Wybierz pierwszy segment (.001) lub plik sumy kontrolnej .crc');
    return;
  }
  document.getElementById('join-file-name').textContent = `Pierwsza część: ${items[0].name}`;
  document.getElementById('join-dest-input').value = getInactivePanelPath();
  openModal('modal-join');
}

async function executeJoin() {
  const items = getActiveSelection();
  const dstDir = document.getElementById('join-dest-input').value;
  closeModal('modal-join');

  try {
    const res = await fetch('/api/join', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ first_part: items[0].path, dst_dir: dstDir })
    });
    if (!res.ok) {
      const err = await res.json();
      alert(`Błąd łączenia: ${err.error}`);
      return;
    }
    refreshBothPanels();
  } catch (err) {
    alert(`Błąd: ${err.message}`);
  }
}

// --- Diff Compare ---
async function compareFilesAction() {
  const leftItems = state.left.selected.size ? Array.from(state.left.selected).map(i => state.left.items[i]) : [state.left.items[state.left.cursor]];
  const rightItems = state.right.selected.size ? Array.from(state.right.selected).map(i => state.right.items[i]) : [state.right.items[state.right.cursor]];

  if (!leftItems[0] || !rightItems[0] || leftItems[0].is_dir || rightItems[0].is_dir) {
    alert('Wybierz pojedynczy plik w lewym i prawym panelu do porównania.');
    return;
  }

  try {
    const res = await fetch('/api/compare', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ file1: leftItems[0].path, file2: rightItems[0].path })
    });
    const data = await res.json();
    if (data.Identical) {
      alert(`Pliki są IDENTYCZNE!\n\n${leftItems[0].name}\n${rightItems[0].name}\nRozmiar: ${data.SizeA} B`);
    } else {
      alert(`Pliki RÓŻNIĄ SIĘ!\n\n${data.Message || ''}\nPrzesunięcie: ${data.DiffOffset} B\nLinia: ${data.DiffLine}`);
    }
  } catch (err) {
    alert(`Błąd porównania: ${err.message}`);
  }
}

// --- Command Bar Execution ---
async function executeCommand() {
  const input = document.getElementById('cmd-input');
  const cmd = input.value.trim();
  if (!cmd) return;
  const p = state[state.activePanel];
  input.value = '';

  try {
    const res = await fetch('/api/exec', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cmd, cwd: p.path })
    });
    const data = await res.json();
    if (data.output) {
      alert(`Wynik polecenia:\n\n${data.output}`);
    }
    refreshBothPanels();
  } catch (err) {
    alert(`Błąd wykonania: ${err.message}`);
  }
}

// --- Modal Utilities ---
function openModal(id) {
  document.getElementById('modal-backdrop').style.display = 'block';
  const el = document.getElementById(id);
  el.style.display = 'flex';
}

function closeModal(id) {
  const el = document.getElementById(id);
  if (el) el.style.display = 'none';
  const anyOpen = document.querySelector('.modal-dialog[style*="display: flex"]');
  if (!anyOpen) {
    document.getElementById('modal-backdrop').style.display = 'none';
  }
}

function closeModals() {
  document.querySelectorAll('.modal-dialog').forEach(el => el.style.display = 'none');
  document.getElementById('modal-backdrop').style.display = 'none';
}

function showAboutDialog() {
  openModal('modal-about');
}

function showHelp() {
  alert('Skróty klawiszowe Total Commander w myc:\n\n' +
    'Tab: Przełączenie panelu\n' +
    'Insert / Spacja: Zaznacz plik (na czerwono) i przejdź niżej\n' +
    '+ / -: Zaznacz / odznacz wg wzorca\n' +
    '*: Odwróć zaznaczenie\n' +
    'Ctrl+A: Zaznacz wszystko\n' +
    'F3: Podgląd (Lister)\n' +
    'F4: Edycja\n' +
    'F5: Kopiuj\n' +
    'F6: Zmień / Przenieś\n' +
    'F7: Nowy katalog\n' +
    'F8: Usuń\n' +
    'Ctrl+F / Alt+F7: Szukaj plików\n' +
    'Ctrl+M: Multi-Rename\n' +
    'Alt+F4: Wyjście z programu');
}

function toggleTheme() {
  document.body.classList.toggle('tc-dark');
}

function exitApp() {
  if (confirm('Czy na pewno chcesz wyjść z programu myc?')) {
    fetch('/api/exit', { method: 'POST' }).finally(() => {
      window.close();
      document.body.innerHTML = '<div style="padding:40px; text-align:center; font-family:sans-serif;"><h3>Aplikacja myc została zamknięta.</h3><p>Możesz zamknąć tę kartę/okno.</p></div>';
    });
  }
}

function refreshActivePanel() {
  loadPanel(state.activePanel);
}

function refreshBothPanels() {
  loadPanel('left');
  loadPanel('right');
}

// --- Formatters ---
function formatBytes(bytes) {
  if (bytes === 0 || bytes === undefined) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatSize(bytes) {
  return formatBytes(bytes);
}

function escapeHTML(str) {
  if (!str) return '';
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
