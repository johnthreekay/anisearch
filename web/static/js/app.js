let allResults = [];
let sortColumn = 'score';
let sortDir = 'desc';
let currentQuery = '';
let activeUser = '';
let nextPage = 2;
let hasMorePages = true;
let lastSearchParams = null; // track search params for load more // current user-scoped filter // default: highest score first

// ── API helper (session cookie handles auth automatically) ──
function apiFetch(url, opts = {}) {
    const headers = { ...opts.headers };
    if (opts.body && typeof opts.body === 'object') {
        headers['Content-Type'] = 'application/json';
        opts.body = JSON.stringify(opts.body);
    }
    return fetch(url, { ...opts, headers, credentials: 'same-origin' }).then(async r => {
        if (r.status === 401) {
            // Session expired, redirect to login
            window.location.href = '/login';
            throw new Error('Session expired');
        }
        const data = await r.json();
        if (!data.success) throw new Error(data.error || 'Unknown error');
        return data.data;
    });
}

// ── Init ──────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    checkStatus();

    const searchInput = document.getElementById('search-input');
    if (searchInput) {
        searchInput.addEventListener('keydown', e => {
            if (e.key === 'Enter') doSearch();
        });
        searchInput.focus();
    }
});

// ── Logout ────────────────────────────────────────
function doLogout() {
    fetch('/api/logout', { method: 'POST', credentials: 'same-origin' })
        .finally(() => {
            window.location.href = '/login';
        });
}

// ── Home / Logo click ─────────────────────────────
function goHome(e) {
    e.preventDefault();
    document.getElementById('search-input').value = '';
    document.getElementById('results-table').classList.add('hidden');
    document.getElementById('results-header').classList.add('hidden');
    document.getElementById('no-results').classList.add('hidden');
    document.getElementById('loading').classList.add('hidden');
    document.getElementById('empty-state').classList.remove('hidden');
    allResults = [];
    currentQuery = '';
    activeUser = '';
    nextPage = 2;
    hasMorePages = true;
    lastSearchParams = null;
    document.getElementById('load-more-area').classList.add('hidden');
    document.getElementById('search-input').focus();
}

// ── Search ────────────────────────────────────────
function doSearch(user) {
    const query = document.getElementById('search-input').value.trim();
    if (!query) return;

    const category = document.getElementById('category-select').value;
    const filter = document.getElementById('filter-select').value;

    // If called without user arg (normal search), clear user filter
    if (user === undefined) user = '';
    activeUser = user;
    currentQuery = query;

    document.getElementById('empty-state').classList.add('hidden');
    document.getElementById('no-results').classList.add('hidden');
    document.getElementById('results-table').classList.add('hidden');
    document.getElementById('results-header').classList.add('hidden');
    document.getElementById('loading').classList.remove('hidden');

    const params = new URLSearchParams({ q: query, c: category, f: filter });
    if (user) params.set('user', user);

    apiFetch(`/api/search?${params}`)
        .then(data => {
            allResults = data.results || [];
            nextPage = 2;
            hasMorePages = data.hasNext;
            lastSearchParams = params;
            document.getElementById('loading').classList.add('hidden');
            populateGroupFilter();
            updateSortArrows();
            renderResults();
            updateLoadMoreButton();
        })
        .catch(err => {
            document.getElementById('loading').classList.add('hidden');
            // If user-scoped search failed, fall back to client-side filtering
            if (user) {
                doSearchWithFallback(user);
            } else {
                toast('Search failed: ' + err.message, 'error');
            }
        });
}

// ── Group filter ──────────────────────────────────
function populateGroupFilter() {
    const select = document.getElementById('group-filter');
    const groups = new Map();

    allResults.forEach(r => {
        if (r.group) {
            groups.set(r.group, (groups.get(r.group) || 0) + 1);
        }
    });

    const sorted = [...groups.entries()].sort((a, b) => b[1] - a[1]);

    select.innerHTML = '<option value="">All groups</option>';
    sorted.forEach(([group, count]) => {
        const opt = document.createElement('option');
        opt.value = group;
        opt.textContent = `${group} (${count})`;
        select.appendChild(opt);
    });

    // Reflect active user filter in dropdown
    select.value = activeUser || '';
}

function onGroupFilterChange() {
    const user = document.getElementById('group-filter').value;
    if (user) {
        doSearch(user);
    } else {
        // "All groups" selected — redo normal search
        doSearch();
    }
}

function filterByGroup(group) {
    if (!currentQuery) return;
    document.getElementById('group-filter').value = group;
    doSearch(group);
}

// Fallback: if user search fails, do a normal search and filter client-side
function doSearchWithFallback(user) {
    const query = document.getElementById('search-input').value.trim();
    if (!query) return;

    const category = document.getElementById('category-select').value;
    const filter = document.getElementById('filter-select').value;

    document.getElementById('empty-state').classList.add('hidden');
    document.getElementById('no-results').classList.add('hidden');
    document.getElementById('results-table').classList.add('hidden');
    document.getElementById('results-header').classList.add('hidden');
    document.getElementById('loading').classList.remove('hidden');

    // Try normal search, then filter client-side by group name
    activeUser = '';
    clientGroupFilter = user;
    const params = new URLSearchParams({ q: query, c: category, f: filter });

    apiFetch(`/api/search?${params}`)
        .then(data => {
            allResults = (data.results || []).filter(r => r.group && r.group.toLowerCase() === user.toLowerCase());
            hasMorePages = false;
            document.getElementById('loading').classList.add('hidden');
            populateGroupFilter();
            updateSortArrows();
            renderResults();
            updateLoadMoreButton();
            toast(`User "${user}" not found on Nyaa, filtered locally instead`, 'info');
        })
        .catch(err => {
            document.getElementById('loading').classList.add('hidden');
            toast('Search failed: ' + err.message, 'error');
        });
}

// ── Sorting ───────────────────────────────────────
function toggleSort(col) {
    if (sortColumn === col) {
        sortDir = sortDir === 'desc' ? 'asc' : 'desc';
    } else {
        sortColumn = col;
        // Default desc for numeric columns, asc for name
        sortDir = col === 'name' ? 'asc' : 'desc';
    }
    updateSortArrows();
    renderResults();
}

function updateSortArrows() {
    document.querySelectorAll('.sort-arrow').forEach(el => el.textContent = '');
    const arrow = document.getElementById('sort-' + sortColumn);
    if (arrow) arrow.textContent = sortDir === 'asc' ? ' ▲' : ' ▼';
}

function sortResults(results) {
    return [...results].sort((a, b) => {
        let va, vb;
        switch (sortColumn) {
            case 'score':     va = a.score || 0;     vb = b.score || 0; break;
            case 'name':      va = (a.title || '').toLowerCase(); vb = (b.title || '').toLowerCase(); break;
            case 'size':      va = a.sizeBytes || 0; vb = b.sizeBytes || 0; break;
            case 'date':      va = new Date(a.date || 0).getTime(); vb = new Date(b.date || 0).getTime(); break;
            case 'seeders':   va = a.seeders || 0;   vb = b.seeders || 0; break;
            case 'leechers':  va = a.leechers || 0;  vb = b.leechers || 0; break;
            case 'downloads': va = a.downloads || 0;  vb = b.downloads || 0; break;
            default: return 0;
        }
        if (sortColumn === 'name') {
            const cmp = va < vb ? -1 : va > vb ? 1 : 0;
            return sortDir === 'asc' ? cmp : -cmp;
        }
        return sortDir === 'asc' ? va - vb : vb - va;
    });
}

// ── Render results ────────────────────────────────
function renderResults() {
    const batchOnly = document.getElementById('batch-only').checked;

    let filtered = allResults;
    if (batchOnly) filtered = filtered.filter(r => r.isBatch);
    filtered = sortResults(filtered);

    if (filtered.length === 0) {
        document.getElementById('no-results').classList.remove('hidden');
        document.getElementById('results-table').classList.add('hidden');
        document.getElementById('results-header').classList.remove('hidden');
        document.getElementById('result-count').textContent = '0 results';
        return;
    }

    document.getElementById('no-results').classList.add('hidden');
    document.getElementById('results-header').classList.remove('hidden');
    document.getElementById('results-table').classList.remove('hidden');
    document.getElementById('result-count').textContent = `${filtered.length} result${filtered.length !== 1 ? 's' : ''}`;

    const tbody = document.getElementById('results-body');
    tbody.innerHTML = '';

    filtered.forEach((r, idx) => {
        const origIdx = allResults.indexOf(r);
        const tr = document.createElement('tr');
        if (r.isBatch) tr.classList.add('is-batch');
        if (r.isTrusted) tr.classList.add('is-trusted');

        let scoreClass = 'score-low';
        if (r.score >= 60) scoreClass = 'score-high';
        else if (r.score >= 35) scoreClass = 'score-mid';

        let tags = '';
        if (r.group) tags += `<span class="tag tag-group clickable" onclick="filterByGroup('${esc(r.group)}')" title="Filter by ${esc(r.group)}">${esc(r.group)}</span>`;
        if (r.resolution) tags += `<span class="tag tag-res">${esc(r.resolution)}</span>`;
        if (r.isBatch) tags += `<span class="tag tag-batch">BATCH</span>`;
        if (r.isTrusted) tags += `<span class="tag tag-trusted">TRUSTED</span>`;

        const date = r.date ? formatDate(r.date) : '—';

        const magnet = r.magnet || (r.infoHash ? `magnet:?xt=urn:btih:${r.infoHash}` : '');
        const torrent = r.torrent || '';

        let titleHtml;
        if (r.link) {
            titleHtml = `<a class="title-link" href="${esc(r.link)}" target="_blank" rel="noopener">${esc(r.title)}</a>`;
        } else {
            titleHtml = `<span class="title-text">${esc(r.title)}</span>`;
        }

        let actionLinks = '';
        if (magnet) actionLinks += `<a class="magnet-link" href="${esc(magnet)}" title="Magnet link">MAG</a>`;
        if (torrent) actionLinks += `<a class="nyaa-link" href="${esc(torrent)}" title="Download .torrent">↗</a>`;

        tr.innerHTML = `
            <td class="col-score"><span class="score-badge ${scoreClass}">${r.score}</span></td>
            <td class="col-name">
                <div class="title-cell">
                    ${titleHtml}
                    <div class="title-tags">${tags}</div>
                </div>
            </td>
            <td class="col-size"><span class="size-text">${esc(r.size || '—')}</span></td>
            <td class="col-date"><span class="date-text">${date}</span></td>
            <td class="col-seed"><span class="seed-count">${r.seeders || 0}</span></td>
            <td class="col-leech"><span class="leech-count">${r.leechers || 0}</span></td>
            <td class="col-dl"><span class="dl-count">${r.downloads || 0}</span></td>
            <td class="col-actions">
                <div class="action-row">
                    <button class="grab-btn" id="grab-${origIdx}" onclick="grabRelease(${origIdx}, this)" ${!magnet && !torrent ? 'disabled' : ''}>Grab</button>
                    ${actionLinks}
                </div>
            </td>
        `;

        tbody.appendChild(tr);
    });
}

function filterResults() {
    renderResults();
}

// ── Load more (pagination) ────────────────────────
function updateLoadMoreButton() {
    const area = document.getElementById('load-more-area');
    const btn = document.getElementById('load-more-btn');
    if (hasMorePages && allResults.length > 0) {
        area.classList.remove('hidden');
        btn.disabled = false;
        btn.textContent = `Load page ${nextPage}`;
    } else {
        area.classList.add('hidden');
    }
}

function loadMore() {
    if (!lastSearchParams || !hasMorePages) return;

    const btn = document.getElementById('load-more-btn');
    const status = document.getElementById('load-more-status');
    btn.disabled = true;
    btn.textContent = `Loading page ${nextPage}...`;

    const params = new URLSearchParams(lastSearchParams);
    params.set('p', nextPage);

    apiFetch(`/api/search/page?${params}`)
        .then(data => {
            const newResults = data.results || [];
            hasMorePages = data.hasNext;
            nextPage = data.page + 1;

            // Deduplicate by infoHash
            const existing = new Set(allResults.map(r => r.infoHash).filter(Boolean));
            const unique = newResults.filter(r => !r.infoHash || !existing.has(r.infoHash));

            allResults = allResults.concat(unique);
            populateGroupFilter();
            renderResults();

            if (unique.length > 0) {
                status.textContent = `${allResults.length} results total`;
            } else {
                hasMorePages = false;
            }

            updateLoadMoreButton();

            if (!hasMorePages) {
                status.textContent = `All ${allResults.length} results loaded`;
            }
        })
        .catch(err => {
            btn.disabled = false;
            btn.textContent = `Load page ${nextPage}`;
            toast('Load more failed: ' + err.message, 'error');
        });
}

// ── Grab ──────────────────────────────────────────
function grabRelease(idx, btn) {
    const r = allResults[idx];
    if (!r) return;

    btn.disabled = true;
    btn.textContent = '...';

    const body = {};
    if (r.magnet) {
        body.magnet = r.magnet;
    } else if (r.infoHash) {
        body.magnet = `magnet:?xt=urn:btih:${r.infoHash}`;
    } else if (r.torrent) {
        body.torrent = r.torrent;
    }

    apiFetch('/api/grab', { method: 'POST', body })
        .then(() => {
            btn.textContent = '✓ Grabbed';
            btn.classList.add('grabbed');
            toast(`Sent to qBittorrent: ${truncate(r.title, 60)}`, 'success');
        })
        .catch(err => {
            btn.disabled = false;
            btn.textContent = 'Grab';
            toast('Grab failed: ' + err.message, 'error');
        });
}

// ── Sonarr rescan ─────────────────────────────────
function triggerRescan() {
    apiFetch('/api/rescan', { method: 'POST' })
        .then(msg => toast('Sonarr rescan triggered', 'success'))
        .catch(err => toast('Rescan failed: ' + err.message, 'error'));
}

// ── Status check ──────────────────────────────────
function checkStatus() {
    apiFetch('/api/status')
        .then(status => {
            setDot('dot-qbit', status.qbit === 'connected');
            setDot('dot-sonarr', status.sonarr === 'connected');
        })
        .catch(() => {
            setDot('dot-qbit', false);
            setDot('dot-sonarr', false);
        });
}

function setDot(id, ok) {
    const el = document.getElementById(id);
    el.classList.remove('ok', 'err');
    el.classList.add(ok ? 'ok' : 'err');
}

setInterval(checkStatus, 60000);

// ── Toast ─────────────────────────────────────────
function toast(msg, type = 'info') {
    const container = document.getElementById('toast-container');
    const el = document.createElement('div');
    el.className = `toast toast-${type}`;
    el.textContent = msg;
    container.appendChild(el);
    setTimeout(() => {
        el.style.opacity = '0';
        el.style.transition = 'opacity 0.3s';
        setTimeout(() => el.remove(), 300);
    }, 4000);
}

// ── Helpers ───────────────────────────────────────
function esc(s) {
    const d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
}

function truncate(s, n) {
    return s.length > n ? s.substring(0, n) + '...' : s;
}

function formatDate(dateStr) {
    const d = new Date(dateStr);
    const now = new Date();
    const diff = now - d;
    const days = Math.floor(diff / 86400000);

    if (days === 0) return 'Today';
    if (days === 1) return 'Yesterday';
    if (days < 30) return `${days}d ago`;
    if (days < 365) return `${Math.floor(days / 30)}mo ago`;
    return `${Math.floor(days / 365)}y ago`;
}
