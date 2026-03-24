/* ═══ STATE ═══ */
/* notes = containers (formerly projects), allMemos = individual documents (formerly notes) */
const S = { notes:[], allMemos:[], memoMap:{}, selected:null, dirty:false, filter:'all', searchQuery:'', collapsed:{} };

/* ═══ SVG ICONS ═══ */
const ICO = {
  chevron: '<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 9l6 6 6-6"/></svg>',
  trash: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M3 6h18M8 6V4a2 2 0 012-2h4a2 2 0 012 2v2M19 6l-1 14a2 2 0 01-2 2H8a2 2 0 01-2-2L5 6"/></svg>',
  x: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6L6 18M6 6l12 12"/></svg>',
};

/* ═══ API (with error handling) ═══ */
const api = {
  async get(p) { try { const r = await fetch(p); return r.json(); } catch(e) { showToast('Network error','error'); return {error:e.message}; } },
  async put(p, b) { try { const r = await fetch(p, {method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)}); return r.json(); } catch(e) { showToast('Network error','error'); return {error:e.message}; } },
  async del(p, b) { try { const o={method:'DELETE'}; if(b){o.headers={'Content-Type':'application/json'};o.body=JSON.stringify(b);} const r=await fetch(p,o); return r.json(); } catch(e) { showToast('Network error','error'); return {error:e.message}; } },
  async post(p, b) { try { const r = await fetch(p, {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(b)}); return r.json(); } catch(e) { showToast('Network error','error'); return {error:e.message}; } },
};

/* ═══ INIT ═══ */
async function init() {
  const data = await api.get('/api/all-data');
  if (data.error) return;
  S.notes = data.notes || [];
  S.allMemos = data.memos || [];
  S.memoMap = {};
  S.allMemos.forEach(m => S.memoMap[m.id] = m);
  initFilterBar();
  renderSidebar();
  updateFooter();
  setupGraph();
  setupEvents();
}

/* ═══ FILTER BAR (event delegation, no inline onclick) ═══ */
function initFilterBar() {
  const bar = document.getElementById('filterBar');
  bar.innerHTML = ['all','concrete','abstract'].map(f =>
    `<button class="filter-btn ${f==='all'?'active':''}" data-filter="${f}" aria-label="Show ${f} memos">${f.charAt(0).toUpperCase()+f.slice(1)}</button>`
  ).join('');
  bar.addEventListener('click', e => {
    const btn = e.target.closest('[data-filter]');
    if (!btn) return;
    S.filter = btn.dataset.filter;
    bar.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    renderSidebar();
  });
}

/* ═══ SIDEBAR (event delegation, data-* attrs, no inline handlers) ═══ */
function getFilteredMemos() {
  let memos = S.allMemos;
  if (S.filter === 'concrete') memos = memos.filter(m => m.layer !== 'abstract');
  if (S.filter === 'abstract') memos = memos.filter(m => m.layer === 'abstract');
  if (S.searchQuery) {
    const q = S.searchQuery.toLowerCase();
    memos = memos.filter(m => m.title.toLowerCase().includes(q) || (m.content||'').toLowerCase().includes(q) || (m.tags||[]).some(t => t.toLowerCase().includes(q)));
  }
  return memos;
}

function renderSidebar() {
  const memos = getFilteredMemos();
  const tree = document.getElementById('sidebarTree');
  /* Group memos by note_id. 0 or missing = Global, others = note containers */
  const groups = {}; const noteMap = {};
  S.notes.forEach(n => { noteMap[n.id] = n; groups[n.id] = []; });
  groups[0] = [];
  memos.forEach(m => { const nid = m.note_id || 0; if (!groups[nid]) groups[nid] = []; groups[nid].push(m); });

  let html = '';
  for (const [nid, noteMemos] of Object.entries(groups)) {
    if (noteMemos.length === 0 && String(nid) !== '0') continue;
    const note = noteMap[nid]; const name = note ? note.name : 'Global';
    const collapsed = S.collapsed[nid] || false;
    html += `<div class="project-section">
      <div class="project-header ${collapsed?'collapsed':''}" data-project-id="${esc(nid)}" tabindex="0" role="button" aria-expanded="${!collapsed}">
        ${ICO.chevron}<span class="p-name">${esc(name)}</span><span class="p-count">${noteMemos.length}</span>
      </div><div class="project-notes ${collapsed?'hidden':''}">`;
    noteMemos.forEach(m => {
      const layer = m.layer === 'abstract' ? 'abstract' : 'concrete';
      const active = S.selected && S.selected.id === m.id ? 'active' : '';
      html += `<div class="note-item ${active}" data-memo-id="${esc(m.id)}" tabindex="0" role="button">
        <div class="note-dot ${layer}"></div><span class="n-title">${esc(trunc(m.title,35))}</span></div>`;
    });
    html += '</div></div>';
  }
  tree.innerHTML = html || '<div style="padding:20px;color:var(--text-muted);font-size:12px;text-align:center">No memos found</div>';
}

function updateFooter() {
  const c = S.allMemos.filter(m => m.layer !== 'abstract').length;
  const a = S.allMemos.length - c;
  document.getElementById('sidebarFooter').textContent = `${S.allMemos.length} memos (${c}C/${a}A) \u00b7 ${S.notes.length} notes`;
}

/* ═══ EDITOR ═══ */
async function selectMemo(id) {
  if (S.dirty && !confirm('Discard unsaved changes?')) return;
  S.dirty = false;
  cacheLayout();
  const memo = await api.get(`/api/memo?id=${encodeURIComponent(id)}`);
  if (memo.error) { showToast(memo.error, 'error'); return; }
  S.selected = memo;
  S.memoMap[memo.id] = memo;
  document.getElementById('editorEmpty').style.display = 'none';
  document.getElementById('editorContent').classList.add('visible');
  document.getElementById('editorId').textContent = memo.id;
  const layerEl = document.getElementById('editorLayer');
  layerEl.textContent = memo.layer;
  layerEl.className = 'layer-badge ' + memo.layer;
  const note = S.notes.find(n => n.id === memo.note_id);
  document.getElementById('editorProject').textContent = note ? note.name : 'Global';
  document.getElementById('titleInput').value = memo.title;
  document.getElementById('contentArea').value = memo.content;
  document.getElementById('saveBar').classList.remove('visible');
  renderTags(memo);
  renderLinks();
  renderMeta(memo);
  document.querySelectorAll('.note-item').forEach(el => el.classList.toggle('active', el.dataset.memoId == id));
  updateGraph(memo.id);
}

function markDirty() {
  if (!S.dirty) { S.dirty = true; document.getElementById('saveBar').classList.add('visible'); }
}

async function saveMemo() {
  if (!S.selected) return;
  const result = await api.put(`/api/memo?id=${encodeURIComponent(S.selected.id)}`, {
    title: document.getElementById('titleInput').value,
    content: document.getElementById('contentArea').value,
  });
  if (result.error) { showToast(result.error, 'error'); return; }
  S.selected = result; S.memoMap[result.id] = result;
  const idx = S.allMemos.findIndex(m => m.id === result.id);
  if (idx >= 0) S.allMemos[idx] = result;
  renderSidebar();
  S.dirty = false;
  document.getElementById('saveBar').classList.remove('visible');
  showToast('Saved', 'success');
}

function renderTags(memo) {
  const area = document.getElementById('tagsArea');
  let html = '';
  (memo.tags || []).forEach(t => {
    html += `<span class="tag-chip">${esc(t)}<span class="tag-remove" data-tag-remove="${esc(t)}" title="Remove tag" role="button" tabindex="0" aria-label="Remove tag ${esc(t)}">${ICO.x}</span></span>`;
  });
  html += `<input class="tag-input" id="tagInput" type="text" placeholder="add tag..." aria-label="Add new tag" />`;
  area.innerHTML = html;
}

async function addTag(tag) {
  if (!S.selected) return;
  const result = await api.post('/api/tag', { memo_id: S.selected.id, tag, action:'add' });
  if (result.error) { showToast(result.error,'error'); return; }
  S.selected = result; S.memoMap[result.id] = result;
  const idx = S.allMemos.findIndex(m => m.id === result.id);
  if (idx >= 0) S.allMemos[idx] = result;
  renderTags(result);
  showToast(`Tag "${tag}" added`, 'success');
}

async function removeTag(tag) {
  if (!S.selected) return;
  const result = await api.post('/api/tag', { memo_id: S.selected.id, tag, action:'remove' });
  if (result.error) { showToast(result.error,'error'); return; }
  S.selected = result; S.memoMap[result.id] = result;
  const idx = S.allMemos.findIndex(m => m.id === result.id);
  if (idx >= 0) S.allMemos[idx] = result;
  renderTags(result);
  showToast(`Tag "${tag}" removed`, 'success');
}

function renderLinks() {
  const area = document.getElementById('linksArea');
  const links = S.selected && S.selected.links;
  if (!links || (links.outgoing.length === 0 && links.incoming.length === 0)) {
    area.innerHTML = '<div class="links-empty">No links</div>';
    return;
  }
  let html = '';
  if (links.outgoing.length > 0) {
    html += '<div class="link-group-label">Outgoing</div>';
    links.outgoing.forEach(l => {
      const target = S.memoMap[l.target_id];
      const title = target ? target.title : 'Memo #' + l.target_id;
      const w = l.weight ? l.weight.toFixed(1) : '';
      html += `<div class="link-item" data-link-nav="${esc(l.target_id)}" role="button" tabindex="0">
        <span class="link-type ${esc(l.relation_type)}">${esc(l.relation_type)}</span>
        <span class="link-title">${esc(trunc(title, 30))}</span>
        ${w ? '<span class="link-weight">' + esc(w) + '</span>' : ''}
        <span class="link-delete" data-link-del-source="${esc(l.source_id)}" data-link-del-target="${esc(l.target_id)}" data-link-del-type="${esc(l.relation_type)}" title="Remove link" role="button" tabindex="0" aria-label="Remove link">${ICO.x}</span>
      </div>`;
    });
  }
  if (links.incoming.length > 0) {
    html += '<div class="link-group-label">Incoming</div>';
    links.incoming.forEach(l => {
      const source = S.memoMap[l.source_id];
      const title = source ? source.title : 'Memo #' + l.source_id;
      const w = l.weight ? l.weight.toFixed(1) : '';
      html += `<div class="link-item" data-link-nav="${esc(l.source_id)}" role="button" tabindex="0">
        <span class="link-type ${esc(l.relation_type)}">${esc(l.relation_type)}</span>
        <span class="link-title">${esc(trunc(title, 30))}</span>
        ${w ? '<span class="link-weight">' + esc(w) + '</span>' : ''}
        <span class="link-delete" data-link-del-source="${esc(l.source_id)}" data-link-del-target="${esc(l.target_id)}" data-link-del-type="${esc(l.relation_type)}" title="Remove link" role="button" tabindex="0" aria-label="Remove link">${ICO.x}</span>
      </div>`;
    });
  }
  area.innerHTML = html;
}

async function deleteLink(source, target, type) {
  if (!confirm(`Remove ${type} link?`)) return;
  const result = await api.del('/api/link', { source: Number(source), target: Number(target), type });
  if (result.error) { showToast(result.error, 'error'); return; }
  /* Re-fetch the memo to update links in state */
  if (S.selected) {
    const memo = await api.get(`/api/memo?id=${encodeURIComponent(S.selected.id)}`);
    if (!memo.error) {
      S.selected = memo; S.memoMap[memo.id] = memo;
      renderLinks();
      updateGraph(memo.id);
    }
  }
  showToast('Link removed', 'success');
}

function renderMeta(memo) {
  const cr = memo.metadata?.created_at ? new Date(memo.metadata.created_at).toLocaleString() : '\u2014';
  const up = memo.metadata?.updated_at ? new Date(memo.metadata.updated_at).toLocaleString() : '\u2014';
  const author = memo.metadata?.author || '\u2014';
  document.getElementById('metaGrid').innerHTML = `
    <span class="meta-label">Created</span><span class="meta-value">${esc(cr)}</span>
    <span class="meta-label">Updated</span><span class="meta-value">${esc(up)}</span>
    <span class="meta-label">Author</span><span class="meta-value">${esc(author)}</span>`;
}

async function deleteCurrentMemo() {
  if (!S.selected) return;
  if (!confirm(`Delete "${S.selected.title}"?`)) return;
  try {
    const result = await api.del(`/api/memo?id=${encodeURIComponent(S.selected.id)}`);
    if (result.error) { showToast(result.error, 'error'); return; }
  } catch (err) { showToast('Failed to delete: ' + err.message, 'error'); return; }
  S.allMemos = S.allMemos.filter(m => m.id !== S.selected.id);
  delete S.memoMap[S.selected.id];
  S.selected = null; S.dirty = false;
  document.getElementById('editorEmpty').style.display = '';
  document.getElementById('editorContent').classList.remove('visible');
  renderSidebar(); updateFooter(); clearGraph();
  showToast('Memo deleted', 'success');
}

async function refreshData() {
  const data = await api.get('/api/all-data');
  if (data.error) return;
  S.notes = data.notes || []; S.allMemos = data.memos || [];
  S.memoMap = {}; S.allMemos.forEach(m => S.memoMap[m.id] = m);
  renderSidebar(); updateFooter();
}

/* ═══ EVENTS (centralized delegation) ═══ */
function setupEvents() {
  const tree = document.getElementById('sidebarTree');
  // Sidebar click delegation
  tree.addEventListener('click', e => {
    const memoEl = e.target.closest('[data-memo-id]');
    if (memoEl) { selectMemo(Number(memoEl.dataset.memoId)); return; }
    const projEl = e.target.closest('[data-project-id]');
    if (projEl) { S.collapsed[projEl.dataset.projectId] = !S.collapsed[projEl.dataset.projectId]; renderSidebar(); return; }
  });
  // Keyboard on sidebar items
  tree.addEventListener('keydown', e => {
    if (e.key === 'Enter' || e.key === ' ') {
      const memoEl = e.target.closest('[data-memo-id]');
      if (memoEl) { e.preventDefault(); selectMemo(Number(memoEl.dataset.memoId)); return; }
      const projEl = e.target.closest('[data-project-id]');
      if (projEl) { e.preventDefault(); S.collapsed[projEl.dataset.projectId] = !S.collapsed[projEl.dataset.projectId]; renderSidebar(); }
    }
  });

  // Editor events
  document.getElementById('titleInput').addEventListener('input', markDirty);
  document.getElementById('contentArea').addEventListener('input', markDirty);
  document.getElementById('saveBtn').addEventListener('click', saveMemo);
  document.getElementById('deleteBtn').addEventListener('click', deleteCurrentMemo);

  // Tags delegation
  document.getElementById('tagsArea').addEventListener('click', e => {
    const rm = e.target.closest('[data-tag-remove]');
    if (rm) removeTag(rm.dataset.tagRemove);
  });
  document.getElementById('tagsArea').addEventListener('keydown', e => {
    if (e.target.id === 'tagInput' && e.key === 'Enter' && e.target.value.trim()) {
      addTag(e.target.value.trim()); e.target.value = '';
    }
    const rm = e.target.closest('[data-tag-remove]');
    if (rm && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); removeTag(rm.dataset.tagRemove); }
  });

  // Links delegation (click to navigate, delete button)
  document.getElementById('linksArea').addEventListener('click', e => {
    const del = e.target.closest('[data-link-del-source]');
    if (del) { e.stopPropagation(); deleteLink(del.dataset.linkDelSource, del.dataset.linkDelTarget, del.dataset.linkDelType); return; }
    const nav = e.target.closest('[data-link-nav]');
    if (nav) selectMemo(Number(nav.dataset.linkNav));
  });
  document.getElementById('linksArea').addEventListener('keydown', e => {
    if (e.key === 'Enter' || e.key === ' ') {
      const del = e.target.closest('[data-link-del-source]');
      if (del) { e.preventDefault(); e.stopPropagation(); deleteLink(del.dataset.linkDelSource, del.dataset.linkDelTarget, del.dataset.linkDelType); return; }
      const nav = e.target.closest('[data-link-nav]');
      if (nav) { e.preventDefault(); selectMemo(Number(nav.dataset.linkNav)); }
    }
  });

  // Search with debounce - uses server-side FTS5 search when query is present
  let searchTimer = null;
  document.getElementById('searchInput').addEventListener('input', e => {
    if (searchTimer) clearTimeout(searchTimer);
    searchTimer = setTimeout(async () => {
      S.searchQuery = e.target.value;
      if (S.searchQuery.trim()) {
        const results = await api.get(`/api/search?q=${encodeURIComponent(S.searchQuery)}`);
        if (!results.error && Array.isArray(results)) {
          S.allMemos = results;
          S.memoMap = {};
          S.allMemos.forEach(m => S.memoMap[m.id] = m);
        }
      } else {
        await refreshData();
      }
      renderSidebar();
    }, 300);
  });

  // Keyboard shortcuts
  document.addEventListener('keydown', e => {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); if (S.dirty) saveMemo(); }
    if (e.key === 'Escape') { document.getElementById('searchInput').value = ''; S.searchQuery = ''; refreshData(); }
  });

  // Beforeunload guard
  window.addEventListener('beforeunload', e => {
    if (S.dirty) { e.preventDefault(); e.returnValue = ''; }
  });
}

/* ═══ TOAST (timer managed) ═══ */
let toastTimer = null;
function showToast(msg, type) {
  if (toastTimer) clearTimeout(toastTimer);
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.className = 'toast ' + type;
  requestAnimationFrame(() => el.classList.add('visible'));
  toastTimer = setTimeout(() => { el.classList.remove('visible'); toastTimer = null; }, 2000);
}

/* ═══ CANVAS GRAPH ENGINE (with convergence stop + layout cache) ═══ */
let canvas, ctx, gNodes=[], gEdges=[], gNodeIdx={};
let gCam={x:0,y:0,zoom:1}, gPhysics=true, gDragging=null, gPanning=false, gPanStart={x:0,y:0};
let gHover=null, gAnimFrame=null;
const layoutCache = {};

const EDGE_COLORS = { supports:'#22C55E', contradicts:'#EF4444', extends:'#3B82F6', abstracts:'#A78BFA', grounds:'#F59E0B', causes:'#F59E0B', 'example-of':'#67E8F9', related:'#64748B', replaces:'#F59E0B', invalidates:'#EF4444' };

function setupGraph() {
  canvas = document.getElementById('graphCanvas'); ctx = canvas.getContext('2d');
  resizeCanvas(); window.addEventListener('resize', resizeCanvas);
  canvas.addEventListener('mousedown', gDown);
  canvas.addEventListener('mousemove', gMove);
  canvas.addEventListener('mouseup', gUp);
  canvas.addEventListener('wheel', gWheel, {passive:false});
}

function resizeCanvas() {
  const r = canvas.parentElement.getBoundingClientRect();
  canvas.width = r.width * devicePixelRatio; canvas.height = r.height * devicePixelRatio;
  canvas.style.width = r.width+'px'; canvas.style.height = r.height+'px';
  ctx.setTransform(devicePixelRatio,0,0,devicePixelRatio,0,0);
  if (gNodes.length) gDraw();
}

function cacheLayout() {
  if (S.selected && gNodes.length) {
    layoutCache[S.selected.id] = gNodes.map(n => ({id:n.id, x:n.x, y:n.y}));
  }
}

function updateGraph(centerId) {
  document.getElementById('graphEmpty').style.display = 'none';
  const W = canvas.clientWidth, H = canvas.clientHeight;
  gNodes=[]; gNodeIdx={}; gEdges=[];

  const memo = S.memoMap[centerId];
  if (!memo) { clearGraph(); return; }

  /* Helper to add a node if not already present */
  function addNode(id, isCenter) {
    if (gNodeIdx[id] !== undefined) return;
    const m = S.memoMap[id];
    if (!m) return;
    const abs = m.layer === 'abstract';
    const cached = layoutCache[centerId];
    let x, y;
    if (cached) {
      const pos = cached.find(p => p.id === id);
      if (pos) { x = pos.x; y = pos.y; }
    }
    if (x === undefined) {
      if (isCenter) { x = W/2; y = H/2; }
      else {
        const angle = (gNodes.length / 8) * Math.PI * 2;
        x = W/2 + Math.cos(angle) * 80;
        y = H/2 + Math.sin(angle) * 80;
      }
    }
    gNodeIdx[id] = gNodes.length;
    gNodes.push({ id, x, y, vx:0, vy:0, r: isCenter ? 14 : 10,
      color: abs ? '#A78BFA' : '#3B82F6', shape: abs ? 'diamond' : 'circle',
      label: trunc(m.title, 18), fullTitle: m.title, isCenter: !!isCenter });
  }

  /* Add center node */
  addNode(centerId, true);

  /* Add neighbors from link data */
  const links = memo.links;
  if (links) {
    (links.outgoing || []).forEach(l => {
      addNode(l.target_id, false);
      if (gNodeIdx[l.target_id] !== undefined) {
        const color = EDGE_COLORS[l.relation_type] || EDGE_COLORS.related;
        gEdges.push({ from: centerId, to: l.target_id, color, width: Math.max(1, (l.weight || 1) * 1.5), label: l.relation_type });
      }
    });
    (links.incoming || []).forEach(l => {
      addNode(l.source_id, false);
      if (gNodeIdx[l.source_id] !== undefined) {
        const color = EDGE_COLORS[l.relation_type] || EDGE_COLORS.related;
        gEdges.push({ from: l.source_id, to: centerId, color, width: Math.max(1, (l.weight || 1) * 1.5), label: l.relation_type });
      }
    });
  }

  gCam = {x:0, y:0, zoom:1};
  if (gNodes.length > 1) {
    restartAnimation();
  } else {
    gPhysics = false;
    gDraw();
  }
}

function clearGraph() {
  gNodes=[]; gEdges=[]; gNodeIdx={};
  if (gAnimFrame) { cancelAnimationFrame(gAnimFrame); gAnimFrame=null; }
  document.getElementById('graphEmpty').style.display = '';
  ctx.clearRect(0,0,canvas.clientWidth,canvas.clientHeight);
}

function gSimulate() {
  if (!gPhysics || gNodes.length===0) return;
  const N = gNodes.length;
  for (let i=0;i<N;i++) for (let j=i+1;j<N;j++) {
    let dx=gNodes[j].x-gNodes[i].x, dy=gNodes[j].y-gNodes[i].y, d=Math.sqrt(dx*dx+dy*dy)||1;
    let f=Math.min(600/(d*d),4);
    gNodes[i].vx-=dx/d*f; gNodes[i].vy-=dy/d*f;
    gNodes[j].vx+=dx/d*f; gNodes[j].vy+=dy/d*f;
  }
  gEdges.forEach(e => {
    const a=gNodes[gNodeIdx[e.from]], b=gNodes[gNodeIdx[e.to]]; if(!a||!b) return;
    let dx=b.x-a.x, dy=b.y-a.y, d=Math.sqrt(dx*dx+dy*dy)||1, f=(d-80)*.006;
    a.vx+=dx/d*f; a.vy+=dy/d*f; b.vx-=dx/d*f; b.vy-=dy/d*f;
  });
  const cx=canvas.clientWidth/2, cy=canvas.clientHeight/2;
  let maxV = 0;
  gNodes.forEach(n => {
    n.vx+=(cx-n.x)*.0004; n.vy+=(cy-n.y)*.0004;
    n.vx*=.82; n.vy*=.82;
    if (gDragging!==n) { n.x+=n.vx; n.y+=n.vy; }
    maxV = Math.max(maxV, Math.abs(n.vx), Math.abs(n.vy));
  });
  if (maxV < 0.1) gPhysics = false;
}

function gDraw() {
  const W=canvas.clientWidth, H=canvas.clientHeight;
  ctx.clearRect(0,0,W,H); ctx.save();
  ctx.translate(W/2+gCam.x,H/2+gCam.y); ctx.scale(gCam.zoom,gCam.zoom); ctx.translate(-W/2,-H/2);
  gEdges.forEach(e => {
    const a=gNodes[gNodeIdx[e.from]], b=gNodes[gNodeIdx[e.to]]; if(!a||!b)return;
    ctx.beginPath(); ctx.moveTo(a.x,a.y); ctx.lineTo(b.x,b.y);
    ctx.strokeStyle=e.color; ctx.globalAlpha=.4; ctx.lineWidth=e.width; ctx.stroke();
    const ang=Math.atan2(b.y-a.y,b.x-a.x), ar=b.r+3, ax=b.x-Math.cos(ang)*ar, ay=b.y-Math.sin(ang)*ar;
    ctx.beginPath(); ctx.moveTo(ax,ay); ctx.lineTo(ax-Math.cos(ang-.4)*4,ay-Math.sin(ang-.4)*4); ctx.lineTo(ax-Math.cos(ang+.4)*4,ay-Math.sin(ang+.4)*4);
    ctx.closePath(); ctx.fillStyle=e.color; ctx.fill();
  });
  ctx.globalAlpha=1;
  gNodes.forEach(n => {
    const sel = S.selected && S.selected.id===n.id;
    if (n.shape==='diamond') { ctx.beginPath(); ctx.moveTo(n.x,n.y-n.r); ctx.lineTo(n.x+n.r,n.y); ctx.lineTo(n.x,n.y+n.r); ctx.lineTo(n.x-n.r,n.y); ctx.closePath(); }
    else { ctx.beginPath(); ctx.arc(n.x,n.y,n.r,0,Math.PI*2); }
    ctx.fillStyle=n.color; ctx.fill();
    if (sel||n.isCenter) { ctx.lineWidth=2.5; ctx.strokeStyle='#F8FAFC'; ctx.stroke(); }
    else if (gHover===n) { ctx.lineWidth=2; ctx.strokeStyle='#94A3B8'; ctx.stroke(); }
    ctx.font='10px system-ui, sans-serif'; ctx.fillStyle='rgba(248,250,252,.8)'; ctx.textAlign='center';
    ctx.fillText(n.label, n.x, n.y+n.r+12);
  });
  ctx.restore();
}

function gTick() {
  gSimulate(); gDraw();
  if (gPhysics || gDragging || gPanning) gAnimFrame = requestAnimationFrame(gTick);
  else gAnimFrame = null;
}

function restartAnimation() {
  gPhysics = true;
  if (!gAnimFrame) gAnimFrame = requestAnimationFrame(gTick);
}

function gScreenToWorld(sx,sy) { const W=canvas.clientWidth,H=canvas.clientHeight; return {x:(sx-W/2-gCam.x)/gCam.zoom+W/2, y:(sy-H/2-gCam.y)/gCam.zoom+H/2}; }
function gHitTest(sx,sy) { const p=gScreenToWorld(sx,sy); for(let i=gNodes.length-1;i>=0;i--){ const n=gNodes[i],dx=p.x-n.x,dy=p.y-n.y; if(dx*dx+dy*dy<(n.r+4)*(n.r+4)) return n; } return null; }

function gDown(e) {
  const r=canvas.getBoundingClientRect(), hit=gHitTest(e.clientX-r.left,e.clientY-r.top);
  if (hit) { gDragging=hit; canvas.style.cursor='grabbing'; restartAnimation(); }
  else { gPanning=true; gPanStart={x:e.clientX-gCam.x,y:e.clientY-gCam.y}; canvas.style.cursor='grabbing'; restartAnimation(); }
}
function gMove(e) {
  const r=canvas.getBoundingClientRect(), sx=e.clientX-r.left, sy=e.clientY-r.top;
  if (gDragging) { const p=gScreenToWorld(sx,sy); gDragging.x=p.x; gDragging.y=p.y; gDragging.vx=0; gDragging.vy=0; return; }
  if (gPanning) { gCam.x=e.clientX-gPanStart.x; gCam.y=e.clientY-gPanStart.y; if(!gAnimFrame){gDraw();} return; }
  const hit=gHitTest(sx,sy); gHover=hit; canvas.style.cursor=hit?'pointer':'default';
  const tip=document.getElementById('tooltip');
  if (hit) { tip.style.display='block'; tip.style.left=(e.clientX+12)+'px'; tip.style.top=(e.clientY-8)+'px'; tip.innerHTML=`<strong>${esc(hit.fullTitle)}</strong><br><span style="color:var(--text-muted)">${esc(hit.id)}</span>`; }
  else tip.style.display='none';
}
function gUp(e) {
  if (gDragging) { const r=canvas.getBoundingClientRect(), hit=gHitTest(e.clientX-r.left,e.clientY-r.top);
    if (hit&&hit===gDragging) { const m=S.memoMap[hit.id]; if(m) selectMemo(m.id); }
    gDragging=null;
  }
  gPanning=false; canvas.style.cursor='default';
}
function gWheel(e) { e.preventDefault(); gCam.zoom=Math.max(.3,Math.min(4,gCam.zoom*(e.deltaY>0?.9:1.1))); if(!gAnimFrame){gDraw();} }

/* ═══ HELPERS ═══ */
function esc(s) { if (s == null) return ''; const d=document.createElement('div'); d.textContent=String(s); return d.innerHTML; }
function trunc(s,n) { return s.length>n ? s.slice(0,n)+'...' : s; }

/* ═══ BOOT ═══ */
init();
