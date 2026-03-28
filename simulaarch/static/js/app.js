const state = {
    nodes: {},
    edges: {},
    selectedId: null,
    selectedType: null,
    selectedIds: new Set(),
    connecting: null,
    transform: { x: 0, y: 0, scale: 1 },
    snapToGrid: false,
    _didPan: false,
    scenarioName: 'Meu Cenário',
};

const undoStack = [];

const NODE_COLORS = {
    client:  '#89b4fa',
    gateway: '#cba6f7',
    service: '#a6e3a1',
    queue:   '#fab387',
    cluster: '#94e2d5',
};

const NODE_LABELS = {
    client:  'Client',
    gateway: 'API Gateway',
    service: 'Service',
    queue:   'Queue',
    cluster: 'K8s Cluster',
};

const NODE_W = 120;
const NODE_H = 56;

function generateId(prefix = 'node') {
    return prefix + '-' + Date.now() + '-' + Math.floor(Math.random() * 10000);
}

function setStatus(msg, type = '') {
    const el = document.getElementById('status-text');
    el.textContent = msg;
    el.className = type;
}

function getDefaultConfig(type) {
    switch (type) {
        case 'client':  return { rps: 100, payloadSizeKB: 1, concurrency: 10 };
        case 'gateway': return { rateLimitRPS: 500, latencyOverheadMs: 5 };
        case 'service': return { cpuCores: 2, ramGB: 2, processTimeMs: 50 };
        case 'queue':   return { throughputMaxMsgsPerSec: 1000, writeLatencyMs: 2 };
        case 'cluster': return { minReplicas: 1, maxReplicas: 5, hpaThreshold: 0.7 };
        default:        return {};
    }
}

function getNodeIcon(type, color) {
    switch (type) {
        case 'client':
            return `<rect x="8" y="4" width="24" height="16" rx="3" fill="${color}"/>
                    <rect x="14" y="20" width="12" height="3" rx="1" fill="${color}"/>
                    <rect x="11" y="23" width="18" height="2" rx="1" fill="${color}"/>
                    <rect x="10" y="6" width="20" height="11" rx="1" fill="#1e1e2e"/>
                    <circle cx="20" cy="11" r="3" fill="${color}"/>`;
        case 'gateway':
            return `<polygon points="20,2 34,10 34,22 20,30 6,22 6,10" fill="${color}"/>
                    <polygon points="20,7 29,12 29,20 20,25 11,20 11,12" fill="#1e1e2e"/>
                    <text x="20" y="21" text-anchor="middle" fill="${color}" font-size="8" font-weight="bold" font-family="monospace">GW</text>`;
        case 'service':
            return `<rect x="6" y="4" width="28" height="24" rx="4" fill="${color}"/>
                    <rect x="10" y="8" width="20" height="16" rx="2" fill="#1e1e2e"/>
                    <rect x="13" y="11" width="5" height="5" rx="1" fill="${color}"/>
                    <rect x="22" y="11" width="5" height="5" rx="1" fill="${color}"/>
                    <rect x="13" y="18" width="14" height="2" rx="1" fill="${color}"/>`;
        case 'queue':
            return `<ellipse cx="20" cy="9" rx="14" ry="5" fill="${color}"/>
                    <rect x="6" y="9" width="28" height="14" fill="${color}"/>
                    <ellipse cx="20" cy="23" rx="14" ry="5" fill="${color}" opacity="0.8"/>
                    <ellipse cx="20" cy="9" rx="14" ry="5" fill="${color}"/>
                    <ellipse cx="20" cy="9" rx="9" ry="3" fill="#1e1e2e"/>`;
        case 'cluster':
            return `<rect x="4" y="4" width="32" height="24" rx="4" fill="none" stroke="${color}" stroke-width="2" stroke-dasharray="4,2"/>
                    <circle cx="13" cy="13" r="4" fill="${color}"/>
                    <circle cx="27" cy="13" r="4" fill="${color}"/>
                    <circle cx="20" cy="23" r="4" fill="${color}"/>
                    <line x1="13" y1="13" x2="27" y2="13" stroke="${color}" stroke-width="1.5"/>
                    <line x1="13" y1="13" x2="20" y2="23" stroke="${color}" stroke-width="1.5"/>
                    <line x1="27" y1="13" x2="20" y2="23" stroke="${color}" stroke-width="1.5"/>`;
        default: return '';
    }
}

// ─── Render Nodes ─────────────────────────────────────────────────────────────

function renderNode(node) {
    const layer = document.getElementById('nodes-layer');
    const existing = document.getElementById(node.id);
    if (existing) existing.remove();

    const color = NODE_COLORS[node.type] || '#cdd6f4';
    const isSelected = state.selectedId === node.id;
    const sim = node._simResult;

    // Stroke color: selected > multi-selected > sim status > node default
    const isMultiSelected = state.selectedIds.has(node.id);
    const strokeColor = isSelected ? '#f9e2af'
        : isMultiSelected ? '#89b4fa'
        : sim ? (sim.status === 'CRITICAL' ? '#f38ba8'
               : sim.status === 'ALERT'    ? '#f9e2af'
               : '#a6e3a1')
        : color;

    // Utilization badge (hidden for client and cluster)
    const badgeHTML = sim && node.type !== 'client' && node.type !== 'cluster' ? (() => {
        const pct = (sim.utilization * 100).toFixed(0);
        const bc = sim.status === 'CRITICAL' ? '#f38ba8'
                 : sim.status === 'ALERT'    ? '#f9e2af'
                 : '#a6e3a1';
        return `<rect x="${NODE_W - 36}" y="-9" width="36" height="14" rx="7" fill="${bc}" fill-opacity="0.92"/>
                <text x="${NODE_W - 18}" y="0" text-anchor="middle" fill="#1e1e2e" font-size="9" font-weight="700"
                      font-family="'Segoe UI', sans-serif">${pct}%</text>`;
    })() : '';

    const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    g.setAttribute('id', node.id);
    const classes = ['canvas-node'];
    if (sim?.status === 'CRITICAL') classes.push('node-critical');
    g.setAttribute('class', classes.join(' '));
    g.setAttribute('transform', `translate(${node.x}, ${node.y})`);
    g.style.cursor = state.connecting ? 'crosshair' : 'pointer';

    g.innerHTML = `
        <rect class="node-bg" x="0" y="0" width="${NODE_W}" height="${NODE_H}"
              rx="8" fill="#1e1e2e"
              stroke="${strokeColor}"
              stroke-width="${isSelected ? 2.5 : 1.5}"/>
        <svg x="6" y="8" width="40" height="40" viewBox="0 0 40 32">
            ${getNodeIcon(node.type, color)}
        </svg>
        <text x="${NODE_W - 8}" y="22" text-anchor="end"
              fill="${color}" font-size="11" font-weight="600"
              font-family="'Segoe UI', sans-serif">${node.label}</text>
        <text x="${NODE_W - 8}" y="36" text-anchor="end"
              fill="#585b70" font-size="9"
              font-family="'Segoe UI', sans-serif">${NODE_LABELS[node.type] || node.type}</text>
        <circle class="node-port" cx="${NODE_W}" cy="${NODE_H / 2}"
                r="5" fill="${color}" stroke="#1e1e2e" stroke-width="1.5"
                style="cursor:crosshair"/>
        ${badgeHTML}
        ${(() => {
            if (node.type === 'service' && node.config?.clusterRef) {
                const cl = state.nodes[node.config.clusterRef];
                if (cl) return `<rect x="4" y="${NODE_H - 13}" width="${NODE_W - 8}" height="11" rx="3"
                                      fill="#94e2d5" fill-opacity="0.15"/>
                                <text x="8" y="${NODE_H - 4}" fill="#94e2d5" font-size="8"
                                      font-family="'Segoe UI', sans-serif">⎔ ${cl.label}</text>`;
            }
            return '';
        })()}
    `;

    // Tooltip
    g.addEventListener('mouseenter', e => showTooltip(node, e.clientX, e.clientY));
    g.addEventListener('mousemove',  e => moveTooltip(e.clientX, e.clientY));
    g.addEventListener('mouseleave', hideTooltip);

    // Click on port → start connecting
    const port = g.querySelector('.node-port');
    port.addEventListener('mousedown', e => {
        e.stopPropagation();
        startConnecting(node.id);
    });

    // Click on node body → finish connecting OR select/drag
    g.addEventListener('mousedown', e => {
        if (e.button !== 0) return;
        if (e.target.classList.contains('node-port')) return;
        e.stopPropagation();

        if (state.connecting) {
            finishConnecting(node.id);
            return;
        }

        // Shift+click: toggle multi-select
        if (e.shiftKey) {
            if (state.selectedIds.has(node.id)) state.selectedIds.delete(node.id);
            else state.selectedIds.add(node.id);
            renderAll();
            return;
        }

        state.selectedIds.clear();
        selectNode(node.id);

        const svgEl = document.getElementById('canvas');
        const svgRect = svgEl.getBoundingClientRect();
        const { x: tx, y: ty, scale } = state.transform;
        const startW = { x: (e.clientX - svgRect.left - tx) / scale, y: (e.clientY - svgRect.top - ty) / scale };
        const dragOffX = startW.x - node.x;
        const dragOffY = startW.y - node.y;
        let dragging = true;

        const onMove = ev => {
            if (!dragging) return;
            hideTooltip();
            const { x: tx2, y: ty2, scale: sc2 } = state.transform;
            let wx = (ev.clientX - svgRect.left - tx2) / sc2 - dragOffX;
            let wy = (ev.clientY - svgRect.top  - ty2) / sc2 - dragOffY;
            if (state.snapToGrid) {
                const G = 24;
                wx = Math.round(wx / G) * G;
                wy = Math.round(wy / G) * G;
            }
            node.x = wx;
            node.y = wy;
            g.setAttribute('transform', `translate(${node.x}, ${node.y})`);
            renderEdges();
        };

        const onUp = () => {
            dragging = false;
            document.removeEventListener('mousemove', onMove);
            document.removeEventListener('mouseup', onUp);
        };

        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
    });

    layer.appendChild(g);
}

// ─── Tooltip ──────────────────────────────────────────────────────────────────

function showTooltip(node, x, y) {
    const tip = document.getElementById('node-tooltip');
    const sim = node._simResult;

    let html = `<div class="tip-title">${node.label}</div>
                <div class="tip-type">${NODE_LABELS[node.type] || node.type}</div>`;

    if (sim) {
        const sc = sim.status === 'CRITICAL' ? '#f38ba8'
                 : sim.status === 'ALERT'    ? '#f9e2af'
                 : '#a6e3a1';
        html += `<hr class="tip-divider"/>
                 <div class="tip-row"><span>Status</span><span style="color:${sc};font-weight:700">${sim.status}</span></div>
                 <div class="tip-row"><span>Utilização</span><span>${(sim.utilization * 100).toFixed(1)}%</span></div>
                 <div class="tip-row"><span>RPS Efetivo</span><span>${sim.effectiveRPS.toFixed(1)}</span></div>
                 <div class="tip-row"><span>Latência</span><span>${sim.latencyMs.toFixed(1)} ms</span></div>`;
        if (sim.metrics?.lagEst > 0)
            html += `<div class="tip-row"><span>Lag Fila</span><span style="color:#f9e2af">${sim.metrics.lagEst.toFixed(1)} msg/s</span></div>`;
        if (sim.metrics?.replicas !== undefined)
            html += `<div class="tip-row"><span>Réplicas</span><span>${sim.metrics.replicas}</span></div>`;
    } else {
        html += `<div class="tip-hint">Execute a simulação para ver métricas.</div>`;
    }

    tip.innerHTML = html;
    tip.style.display = 'block';
    moveTooltip(x, y);
}

function moveTooltip(x, y) {
    const tip = document.getElementById('node-tooltip');
    let left = x + 14;
    let top  = y + 14;
    if (left + 190 > window.innerWidth)  left = x - 190 - 14;
    if (top  + 160 > window.innerHeight) top  = y - 160 - 14;
    tip.style.left = left + 'px';
    tip.style.top  = top  + 'px';
}

function hideTooltip() {
    document.getElementById('node-tooltip').style.display = 'none';
}

// ─── Render Edges ─────────────────────────────────────────────────────────────

function renderEdges() {
    const layer = document.getElementById('edges-layer');
    layer.innerHTML = '';

    // Max RPS across all simulated edges (for proportional width scaling)
    const allRPS = Object.values(state.edges).map(e => e._simResult?.rpsFlow ?? 0);
    const maxRPS = Math.max(...allRPS, 1);

    Object.values(state.edges).forEach(edge => {
        const from = state.nodes[edge.from];
        const to   = state.nodes[edge.to];
        if (!from || !to) return;

        const x1 = from.x + NODE_W;
        const y1 = from.y + NODE_H / 2;
        const x2 = to.x;
        const y2 = to.y + NODE_H / 2;
        const mx = (x1 + x2) / 2;

        const isAsync    = edge.type === 'async';
        const isSelected = state.selectedId === edge.id;
        const simResult  = edge._simResult;

        let color, strokeWidth;
        if (isSelected) {
            color = '#f9e2af';
            strokeWidth = 3;
        } else if (simResult) {
            const ratio = simResult.rpsFlow / maxRPS;
            color = ratio >= 0.75 ? '#f38ba8'
                  : ratio >= 0.40 ? '#f9e2af'
                  : simResult.rpsFlow > 0 ? '#a6e3a1'
                  : '#313244';
            strokeWidth = Math.max(1.5, Math.min(5, 1.5 + ratio * 3.5));
        } else {
            color = isAsync ? '#fab387' : '#6c7086';
            strokeWidth = 2;
        }

        const marker = isAsync ? 'url(#arrowhead-async)' : 'url(#arrowhead)';

        const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        path.setAttribute('d', `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`);
        path.setAttribute('fill', 'none');
        path.setAttribute('stroke', color);
        path.setAttribute('stroke-width', strokeWidth);
        if (isAsync) path.setAttribute('stroke-dasharray', '6,3');
        path.setAttribute('marker-end', marker);
        path.style.cursor = 'pointer';

        // Invisible wider hit area
        const hitPath = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        hitPath.setAttribute('d', `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`);
        hitPath.setAttribute('fill', 'none');
        hitPath.setAttribute('stroke', 'transparent');
        hitPath.setAttribute('stroke-width', '12');
        hitPath.style.cursor = 'pointer';
        hitPath.addEventListener('click', ev => {
            ev.stopPropagation();
            selectNode(edge.id, 'edge');
        });

        layer.appendChild(path);
        layer.appendChild(hitPath);
    });
}

// ─── Cluster Containers ───────────────────────────────────────────────────────

function renderClusterContainers() {
    let layer = document.getElementById('clusters-layer');
    if (!layer) {
        layer = document.createElementNS('http://www.w3.org/2000/svg', 'g');
        layer.setAttribute('id', 'clusters-layer');
        const viewport = document.getElementById('viewport');
        viewport.insertBefore(layer, viewport.firstChild);
    }
    layer.innerHTML = '';

    const clusterColor = NODE_COLORS.cluster;
    const PAD = 20;

    Object.values(state.nodes)
        .filter(n => n.type === 'cluster')
        .forEach(cluster => {
            const members = Object.values(state.nodes).filter(
                n => n.type === 'service' && n.config?.clusterRef === cluster.id
            );
            if (members.length === 0) return;

            const minX = Math.min(...members.map(n => n.x)) - PAD;
            const minY = Math.min(...members.map(n => n.y)) - PAD - 16;
            const maxX = Math.max(...members.map(n => n.x + NODE_W)) + PAD;
            const maxY = Math.max(...members.map(n => n.y + NODE_H)) + PAD;

            const rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
            rect.setAttribute('x', minX);
            rect.setAttribute('y', minY);
            rect.setAttribute('width', maxX - minX);
            rect.setAttribute('height', maxY - minY);
            rect.setAttribute('rx', '12');
            rect.setAttribute('fill', clusterColor);
            rect.setAttribute('fill-opacity', '0.05');
            rect.setAttribute('stroke', clusterColor);
            rect.setAttribute('stroke-width', '1.5');
            rect.setAttribute('stroke-dasharray', '6,3');
            layer.appendChild(rect);

            const label = document.createElementNS('http://www.w3.org/2000/svg', 'text');
            label.setAttribute('x', minX + 10);
            label.setAttribute('y', minY + 13);
            label.setAttribute('fill', clusterColor);
            label.setAttribute('font-size', '10');
            label.setAttribute('font-family', "'Segoe UI', sans-serif");
            label.setAttribute('font-weight', '600');
            label.textContent = `⎔ ${cluster.label}`;
            layer.appendChild(label);
        });
}

function renderAll() {
    renderClusterContainers();
    Object.values(state.nodes).forEach(renderNode);
    renderEdges();
}

// ─── Connecting Mode ──────────────────────────────────────────────────────────

function startConnecting(fromId) {
    state.connecting = { fromId };
    svgCanvas.style.cursor = 'crosshair';
    setStatus('Clique em outro nó para criar a conexão. ESC para cancelar.', 'warn');

    const from = state.nodes[fromId];
    const pendingLine = document.createElementNS('http://www.w3.org/2000/svg', 'line');
    pendingLine.setAttribute('id', 'pending-edge');
    pendingLine.setAttribute('stroke', '#f9e2af');
    pendingLine.setAttribute('stroke-width', '2');
    pendingLine.setAttribute('stroke-dasharray', '6,3');
    pendingLine.setAttribute('pointer-events', 'none');
    document.getElementById('edges-layer').appendChild(pendingLine);

    const onMouseMove = e => {
        if (!state.connecting) return;
        const wp = screenToWorld(e.clientX, e.clientY);
        pendingLine.setAttribute('x1', from.x + NODE_W);
        pendingLine.setAttribute('y1', from.y + NODE_H / 2);
        pendingLine.setAttribute('x2', wp.x);
        pendingLine.setAttribute('y2', wp.y);
    };

    state.connecting._onMouseMove = onMouseMove;
    document.addEventListener('mousemove', onMouseMove);
}

function finishConnecting(toId) {
    if (!state.connecting) return;
    const { fromId, _onMouseMove } = state.connecting;

    document.removeEventListener('mousemove', _onMouseMove);
    document.getElementById('pending-edge')?.remove();
    state.connecting = null;
    svgCanvas.style.cursor = '';

    if (fromId === toId) {
        setStatus('Não é possível conectar um nó a ele mesmo.', 'error');
        return;
    }

    const duplicate = Object.values(state.edges).find(
        e => e.from === fromId && e.to === toId
    );
    if (duplicate) {
        setStatus('Já existe uma conexão entre esses nós.', 'error');
        return;
    }

    const fromType = state.nodes[fromId]?.type;
    const toType   = state.nodes[toId]?.type;
    const edgeType = (fromType === 'queue' || toType === 'queue') ? 'async' : 'sync';

    const id = generateId('edge');
    state.edges[id] = {
        id,
        from: fromId,
        to:   toId,
        type: edgeType,
        trafficShare: 1.0,
        config: {},
    };

    renderAll();
    selectNode(id, 'edge');
    setStatus(`Conexão criada (${edgeType}).`, 'success');
}

function cancelConnecting() {
    if (!state.connecting) return;
    document.removeEventListener('mousemove', state.connecting._onMouseMove);
    document.getElementById('pending-edge')?.remove();
    state.connecting = null;
    svgCanvas.style.cursor = '';
    setStatus('Conexão cancelada.', 'warn');
    renderAll();
}

// ─── Selection ────────────────────────────────────────────────────────────────

function selectNode(id, type = 'node') {
    state.selectedId = id;
    state.selectedType = type;
    renderAll();
    showProperties(id, type);
}

function deselectAll() {
    state.selectedId = null;
    state.selectedType = null;
    state.selectedIds.clear();
    renderAll();
    document.getElementById('properties-panel').innerHTML =
        '<p id="properties-placeholder">Selecione um componente</p>';
}

// ─── Properties Panel ─────────────────────────────────────────────────────────

// ─── Properties Panel ─────────────────────────────────────────────────────────

function fieldHTML(id, label, value, type = 'number', { min = 0, step = 1, isFloat = false } = {}) {
    return `
        <label class="prop-label">${label}</label>
        <input id="${id}" type="${type}" class="prop-input"
               value="${value}" min="${min}" step="${isFloat ? 0.01 : step}"/>
    `;
}

function bindConfig(inputId, node, key, { isFloat = false } = {}) {
    const el = document.getElementById(inputId);
    if (!el) return;
    el.addEventListener('input', e => {
        const raw = e.target.value;
        let val = isFloat ? parseFloat(raw) : parseInt(raw, 10);
        if (isNaN(val) || val < 0) { e.target.classList.add('input-error'); return; }
        e.target.classList.remove('input-error');
        node.config[key] = val;
    });
}

function configFieldsHTML(node) {
    const c = node.config;
    switch (node.type) {
        case 'client':
            return `
                <p class="prop-section-title" style="margin-top:12px">Configuração</p>
                ${fieldHTML('cfg-rps',         'RPS (req/s)',              c.rps         ?? 100)}
                ${fieldHTML('cfg-payload',     'Payload Size (KB)',        c.payloadSizeKB ?? 1, 'number', { isFloat: true })}
                ${fieldHTML('cfg-concurrency', 'Concurrency (conexões)',   c.concurrency ?? 10)}
            `;
        case 'gateway':
            return `
                <p class="prop-section-title" style="margin-top:12px">Configuração</p>
                ${fieldHTML('cfg-ratelimit',  'Rate Limit RPS',      c.rateLimitRPS     ?? 500)}
                ${fieldHTML('cfg-latency-oh', 'Latency Overhead (ms)', c.latencyOverheadMs ?? 5)}
            `;
        case 'service': {
            const clusters = Object.values(state.nodes).filter(n => n.type === 'cluster');
            const clusterOptions = clusters.map(cl =>
                `<option value="${cl.id}" ${c.clusterRef === cl.id ? 'selected' : ''}>${cl.label}</option>`
            ).join('');
            return `
                <p class="prop-section-title" style="margin-top:12px">Configuração</p>
                ${fieldHTML('cfg-cpu',         'CPU Cores',            c.cpuCores      ?? 2)}
                ${fieldHTML('cfg-ram',         'RAM (GB)',              c.ramGB         ?? 2)}
                ${fieldHTML('cfg-processtime', 'Process Time (ms)',     c.processTimeMs ?? 50)}
                <p class="prop-hint">MaxRPS teórico: <strong id="derived-maxrps">—</strong></p>
                <p class="prop-section-title" style="margin-top:12px">Cluster K8s</p>
                <label class="prop-label">Associar ao Cluster</label>
                <select id="cfg-cluster-ref" class="prop-input">
                    <option value="">— Nenhum —</option>
                    ${clusterOptions}
                </select>
            `;
        }
        case 'queue':
            return `
                <p class="prop-section-title" style="margin-top:12px">Configuração</p>
                ${fieldHTML('cfg-throughput',    'Throughput Max (msgs/s)', c.throughputMaxMsgsPerSec ?? 1000)}
                ${fieldHTML('cfg-write-latency', 'Write Latency (ms)',      c.writeLatencyMs          ?? 2)}
            `;
        case 'cluster':
            return `
                <p class="prop-section-title" style="margin-top:12px">Configuração</p>
                ${fieldHTML('cfg-min-replicas', 'Min Replicas',       c.minReplicas  ?? 1)}
                ${fieldHTML('cfg-max-replicas', 'Max Replicas',       c.maxReplicas  ?? 5)}
                ${fieldHTML('cfg-hpa',          'HPA Threshold (0–1)', c.hpaThreshold ?? 0.7, 'number', { isFloat: true })}
            `;
        default: return '';
    }
}

function bindConfigFields(node) {
    switch (node.type) {
        case 'client':
            bindConfig('cfg-rps',         node, 'rps');
            bindConfig('cfg-payload',     node, 'payloadSizeKB', { isFloat: true });
            bindConfig('cfg-concurrency', node, 'concurrency');
            break;
        case 'gateway':
            bindConfig('cfg-ratelimit',  node, 'rateLimitRPS');
            bindConfig('cfg-latency-oh', node, 'latencyOverheadMs');
            break;
        case 'service':
            bindConfig('cfg-cpu',         node, 'cpuCores');
            bindConfig('cfg-ram',         node, 'ramGB');
            bindConfig('cfg-processtime', node, 'processTimeMs');
            ['cfg-cpu', 'cfg-processtime'].forEach(id => {
                document.getElementById(id)?.addEventListener('input', () => {
                    const cores = parseFloat(document.getElementById('cfg-cpu')?.value) || node.config.cpuCores;
                    const pt    = parseFloat(document.getElementById('cfg-processtime')?.value) || node.config.processTimeMs;
                    const maxRPS = pt > 0 ? ((1000 * cores) / pt).toFixed(1) : '—';
                    const el = document.getElementById('derived-maxrps');
                    if (el) el.textContent = maxRPS + ' RPS';
                });
            });
            const initCores = node.config.cpuCores || 2;
            const initPt    = node.config.processTimeMs || 50;
            const initMax   = initPt > 0 ? ((1000 * initCores) / initPt).toFixed(1) : '—';
            const maxEl = document.getElementById('derived-maxrps');
            if (maxEl) maxEl.textContent = initMax + ' RPS';
            document.getElementById('cfg-cluster-ref')?.addEventListener('change', e => {
                node.config.clusterRef = e.target.value || null;
                renderAll();
            });
            break;
        case 'queue':
            bindConfig('cfg-throughput',    node, 'throughputMaxMsgsPerSec');
            bindConfig('cfg-write-latency', node, 'writeLatencyMs');
            break;
        case 'cluster':
            bindConfig('cfg-min-replicas', node, 'minReplicas');
            bindConfig('cfg-max-replicas', node, 'maxReplicas');
            bindConfig('cfg-hpa',          node, 'hpaThreshold', { isFloat: true });
            break;
    }
}

function simResultHTML(node) {
    const r = node._simResult;
    if (!r) {
        return `<div class="sim-result-empty"><span>Execute a simulação para ver as métricas deste nó.</span></div>`;
    }

    const statusColor = r.status === 'CRITICAL' ? '#f38ba8'
                      : r.status === 'ALERT'    ? '#f9e2af'
                      : '#a6e3a1';
    const utilizationPct = (r.utilization * 100).toFixed(1);

    let rows = `
        <div class="sim-result-entry">
            <span>Status</span>
            <span style="color:${statusColor};font-weight:700">${r.status}</span>
        </div>
        <div class="sim-result-entry">
            <span>Utilização</span>
            <span>${utilizationPct}%</span>
        </div>
        <div class="sim-result-entry">
            <span>RPS efetivo</span>
            <span>${r.effectiveRPS.toFixed(1)}</span>
        </div>
        <div class="sim-result-entry">
            <span>Latência acum.</span>
            <span>${r.latencyMs.toFixed(1)} ms</span>
        </div>
    `;

    if (r.metrics) {
        if (r.metrics.replicas !== undefined) {
            rows += `<div class="sim-result-entry"><span>Réplicas</span><span>${r.metrics.replicas}</span></div>`;
        }
        if (r.metrics.isSaturated !== undefined) {
            rows += `<div class="sim-result-entry"><span>Saturado</span><span>${r.metrics.isSaturated ? 'Sim' : 'Não'}</span></div>`;
        }
        if (r.metrics.ingressRPS !== undefined) {
            rows += `<div class="sim-result-entry"><span>Ingress RPS</span><span>${r.metrics.ingressRPS.toFixed(1)}</span></div>`;
        }
        if (r.metrics.egressRPS !== undefined) {
            rows += `<div class="sim-result-entry"><span>Egress RPS</span><span>${r.metrics.egressRPS.toFixed(1)}</span></div>`;
        }
        if (r.metrics.lagEst !== undefined && r.metrics.lagEst > 0) {
            rows += `<div class="sim-result-entry"><span>Lag estimado</span><span>${r.metrics.lagEst.toFixed(1)} msg/s</span></div>`;
        }
    }

    return `<div class="sim-result-panel">${rows}</div>`;
}

function showProperties(id, type) {
    const panel = document.getElementById('properties-panel');

    if (type === 'node') {
        const node = state.nodes[id];
        if (!node) return;

        panel.innerHTML = `
            <p class="prop-section-title">Propriedades</p>

            <label class="prop-label">Nome</label>
            <input id="prop-label-input" type="text" class="prop-input" value="${node.label}"/>

            <p class="prop-type-badge">Tipo: <strong>${NODE_LABELS[node.type]}</strong></p>

            ${configFieldsHTML(node)}

            <hr class="prop-divider"/>

            <p class="prop-section-title">Resultado da Simulação</p>
            ${simResultHTML(node)}

            <hr class="prop-divider"/>
            <button id="btn-delete-node" class="btn-danger">Remover nó</button>
        `;

        document.getElementById('prop-label-input').addEventListener('input', e => {
            node.label = e.target.value;
            renderNode(node);
        });

        bindConfigFields(node);

        document.getElementById('btn-delete-node').addEventListener('click', () => deleteSelected());

    } else if (type === 'edge') {
        const edge = state.edges[id];
        if (!edge) return;
        const fromLabel = state.nodes[edge.from]?.label || edge.from;
        const toLabel   = state.nodes[edge.to]?.label   || edge.to;

        panel.innerHTML = `
            <p class="prop-section-title">Conexão</p>
            <p class="prop-type-badge" style="margin-bottom:10px">
                <strong>${fromLabel}</strong> → <strong>${toLabel}</strong>
            </p>

            <label class="prop-label">Tipo</label>
            <select id="prop-edge-type" class="prop-input">
                <option value="sync"  ${edge.type === 'sync'  ? 'selected' : ''}>Síncrono (Request/Response)</option>
                <option value="async" ${edge.type === 'async' ? 'selected' : ''}>Assíncrono (Pub/Sub)</option>
            </select>

            <label class="prop-label">Traffic Share (0.0 – 1.0)</label>
            <input id="prop-traffic" type="number" class="prop-input"
                   min="0" max="1" step="0.1" value="${edge.trafficShare}"/>

            <label class="prop-label">Latência por aresta (ms)</label>
            <input id="prop-edge-latency" type="number" class="prop-input"
                   min="0" step="1" value="${edge.config.latencyMs ?? ''}"/>

            <hr class="prop-divider"/>
            <button id="btn-delete-edge" class="btn-danger">Remover conexão</button>
        `;

        document.getElementById('prop-edge-type').addEventListener('change', e => {
            edge.type = e.target.value;
            renderEdges();
        });
        document.getElementById('prop-traffic').addEventListener('input', e => {
            const v = parseFloat(e.target.value);
            if (!isNaN(v) && v >= 0 && v <= 1) edge.trafficShare = v;
        });
        document.getElementById('prop-edge-latency').addEventListener('input', e => {
            const v = parseFloat(e.target.value);
            if (!isNaN(v) && v >= 0) edge.config.latencyMs = v;
            else delete edge.config.latencyMs;
        });
        document.getElementById('btn-delete-edge').addEventListener('click', () => deleteSelected());
    }
}

// ─── Delete ───────────────────────────────────────────────────────────────────

function deleteSelected() {
    if (!state.selectedId) return;

    if (state.selectedType === 'node') {
        const id = state.selectedId;
        const deletedEdges = Object.values(state.edges)
            .filter(e => e.from === id || e.to === id)
            .map(e => ({ ...e, config: { ...e.config } }));
        undoStack.push({
            type: 'deleteNode',
            node: { ...state.nodes[id], config: { ...state.nodes[id].config } },
            edges: deletedEdges,
        });
        if (undoStack.length > 50) undoStack.shift();

        if (state.nodes[id]?.type === 'cluster') {
            Object.values(state.nodes).forEach(n => {
                if (n.config?.clusterRef === id) delete n.config.clusterRef;
            });
        }
        delete state.nodes[id];
        deletedEdges.forEach(e => delete state.edges[e.id]);
        setStatus('Nó removido. Ctrl+Z para desfazer.', 'warn');
    } else if (state.selectedType === 'edge') {
        const e = state.edges[state.selectedId];
        undoStack.push({ type: 'deleteEdge', edge: { ...e, config: { ...e.config } } });
        if (undoStack.length > 50) undoStack.shift();
        delete state.edges[state.selectedId];
        setStatus('Conexão removida. Ctrl+Z para desfazer.', 'warn');
    }

    deselectAll();
}

// ─── Palette Drag ─────────────────────────────────────────────────────────────

document.querySelectorAll('.palette-item').forEach(item => {
    item.addEventListener('dragstart', e => {
        const type = item.dataset.type;
        e.dataTransfer.setData('text/plain', type);
        e.dataTransfer.setData('node-type', type);
        e.dataTransfer.effectAllowed = 'copy';
        item.classList.add('dragging');
    });
    item.addEventListener('dragend', () => item.classList.remove('dragging'));
});

// ─── Drop on Canvas ───────────────────────────────────────────────────────────

const canvasContainer = document.getElementById('canvas-container');
const svgCanvas = document.getElementById('canvas');

[canvasContainer, svgCanvas].forEach(el => {
    el.addEventListener('dragover', e => {
        e.preventDefault();
        e.dataTransfer.dropEffect = 'copy';
    });

    el.addEventListener('drop', e => {
        e.preventDefault();
        e.stopPropagation();
        const type = e.dataTransfer.getData('node-type') || e.dataTransfer.getData('text/plain');
        if (!type || !NODE_LABELS[type]) return;

        const wp = screenToWorld(e.clientX, e.clientY);
        const G = 24;
        let x = wp.x - NODE_W / 2;
        let y = wp.y - NODE_H / 2;
        if (state.snapToGrid) {
            x = Math.round(x / G) * G;
            y = Math.round(y / G) * G;
        }
        const id = generateId('node');

        const existingCount = Object.values(state.nodes).filter(n => n.type === type).length;

        state.nodes[id] = {
            id,
            type,
            label: `${NODE_LABELS[type]} ${existingCount + 1}`,
            x: Math.max(0, x),
            y: Math.max(0, y),
            config: getDefaultConfig(type),
        };

        renderNode(state.nodes[id]);
        selectNode(id);
        setStatus(`Nó "${state.nodes[id].label}" adicionado ao canvas.`, 'success');
    });
});

// ─── Viewport transform helpers ───────────────────────────────────────────────

function applyTransform() {
    const { x, y, scale } = state.transform;
    document.getElementById('viewport').setAttribute('transform', `translate(${x}, ${y}) scale(${scale})`);
}

function screenToWorld(clientX, clientY) {
    const rect = document.getElementById('canvas').getBoundingClientRect();
    const { x, y, scale } = state.transform;
    return { x: (clientX - rect.left - x) / scale, y: (clientY - rect.top - y) / scale };
}

// ─── Zoom (mouse wheel) ───────────────────────────────────────────────────────

svgCanvas.addEventListener('wheel', e => {
    e.preventDefault();
    const delta = e.deltaY < 0 ? 1.1 : 0.9;
    const { x, y, scale } = state.transform;
    const rect = svgCanvas.getBoundingClientRect();
    const mx = e.clientX - rect.left;
    const my = e.clientY - rect.top;
    const newScale = Math.min(3, Math.max(0.15, scale * delta));
    state.transform = {
        x: mx - (mx - x) * (newScale / scale),
        y: my - (my - y) * (newScale / scale),
        scale: newScale,
    };
    applyTransform();
}, { passive: false });

// ─── Pan (right-click drag or Space + left drag) ───────────────────────────────

svgCanvas.addEventListener('mousedown', e => {
    if (e.button !== 2 && !state._spaceDown) return;
    e.preventDefault();
    state._panning = false;
    const startX = e.clientX, startY = e.clientY;
    const startTx = state.transform.x, startTy = state.transform.y;

    const onMove = ev => {
        state._panning = true;
        state.transform.x = startTx + (ev.clientX - startX);
        state.transform.y = startTy + (ev.clientY - startY);
        applyTransform();
    };
    const onUp = () => {
        svgCanvas.style.cursor = state._spaceDown ? 'grab' : '';
        setTimeout(() => { state._panning = false; }, 0);
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
    };
    svgCanvas.style.cursor = 'grabbing';
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
});

svgCanvas.addEventListener('contextmenu', e => e.preventDefault());

// ─── Canvas click = deselect / cancel connecting ──────────────────────────────

svgCanvas.addEventListener('click', e => {
    if (state._panning) return;
    if (e.target === svgCanvas || e.target.id === 'viewport') {
        if (state.connecting) cancelConnecting();
        else deselectAll();
    }
});

// ─── Keyboard ─────────────────────────────────────────────────────────────────

document.addEventListener('keydown', e => {
    // Ctrl+Z: undo
    if ((e.ctrlKey || e.metaKey) && e.key === 'z') {
        e.preventDefault();
        if (undoStack.length === 0) { setStatus('Nada para desfazer.', 'warn'); return; }
        const action = undoStack.pop();
        if (action.type === 'deleteNode') {
            state.nodes[action.node.id] = action.node;
            action.edges.forEach(ed => { state.edges[ed.id] = ed; });
        } else if (action.type === 'deleteEdge') {
            state.edges[action.edge.id] = action.edge;
        }
        renderAll();
        setStatus('Ação desfeita.', 'success');
        return;
    }
    // Ctrl+S: export
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        exportJSON();
        return;
    }
    if (e.key === 'Escape') {
        cancelConnecting();
        return;
    }
    // Space: enable pan cursor
    if (e.code === 'Space' && !e.repeat && document.activeElement.tagName !== 'INPUT') {
        state._spaceDown = true;
        svgCanvas.style.cursor = 'grab';
        e.preventDefault();
        return;
    }
    if ((e.key === 'Delete' || e.key === 'Backspace') &&
        document.activeElement.tagName !== 'INPUT' &&
        document.activeElement.tagName !== 'SELECT') {
        deleteSelected();
    }
});

document.addEventListener('keyup', e => {
    if (e.code === 'Space') {
        state._spaceDown = false;
        if (!state._panning) svgCanvas.style.cursor = '';
    }
});

// ─── Toolbar ──────────────────────────────────────────────────────────────────

document.getElementById('btn-new').addEventListener('click', () => {
    if (Object.keys(state.nodes).length > 0) {
        if (!confirm('Criar novo diagrama? O conteúdo atual será perdido.')) return;
    }
    Object.keys(state.nodes).forEach(k => delete state.nodes[k]);
    Object.keys(state.edges).forEach(k => delete state.edges[k]);
    setScenarioName('Meu Cenário');
    document.getElementById('canvas-legend')?.classList.remove('visible');
    deselectAll();
    setStatus('Novo diagrama criado.', 'success');
});

document.getElementById('btn-clear').addEventListener('click', () => {
    if (!confirm('Limpar canvas?')) return;
    Object.keys(state.nodes).forEach(k => delete state.nodes[k]);
    Object.keys(state.edges).forEach(k => delete state.edges[k]);
    deselectAll();
    setStatus('Canvas limpo.', 'warn');
});

document.getElementById('btn-fit').addEventListener('click', () => {
    const nodes = Object.values(state.nodes);
    if (nodes.length === 0) { setStatus('Canvas vazio — sem conteúdo para centralizar.', 'warn'); return; }
    const minX = Math.min(...nodes.map(n => n.x));
    const minY = Math.min(...nodes.map(n => n.y));
    const maxX = Math.max(...nodes.map(n => n.x + NODE_W));
    const maxY = Math.max(...nodes.map(n => n.y + NODE_H));
    const PADDING = 60;
    const svgRect = svgCanvas.getBoundingClientRect();
    const scale = Math.min(
        (svgRect.width  - PADDING * 2) / Math.max(maxX - minX, 1),
        (svgRect.height - PADDING * 2) / Math.max(maxY - minY, 1),
        1.5
    );
    state.transform = {
        x: (svgRect.width  - (maxX - minX) * scale) / 2 - minX * scale,
        y: (svgRect.height - (maxY - minY) * scale) / 2 - minY * scale,
        scale,
    };
    applyTransform();
    setStatus('Canvas centralizado.', 'success');
});

document.getElementById('btn-snap').addEventListener('click', function () {
    state.snapToGrid = !state.snapToGrid;
    this.textContent = `Snap: ${state.snapToGrid ? 'ON' : 'OFF'}`;
    this.style.color = state.snapToGrid ? '#a6e3a1' : '';
    setStatus(`Snap to grid ${state.snapToGrid ? 'ativado' : 'desativado'}.`, state.snapToGrid ? 'success' : 'warn');
});

// ─── Simulação ────────────────────────────────────────────────────────────────

// Mapeia config frontend (camelCase) → chaves esperadas pelo backend Go
function serializeNodeConfig(node) {
    const c = node.config;
    switch (node.type) {
        case 'client':
            return { RPS: c.rps ?? 100, PayloadSizeKB: c.payloadSizeKB ?? 1, Concurrency: c.concurrency ?? 10 };
        case 'gateway':
            return { RateLimitRPS: c.rateLimitRPS ?? 500, LatencyOverheadMs: c.latencyOverheadMs ?? 5 };
        case 'service': {
            const cfg = { CPU_Cores: c.cpuCores ?? 2, RAM_GB: c.ramGB ?? 2, ProcessTimeMs: c.processTimeMs ?? 50 };
            if (c.clusterRef) cfg.clusterRef = c.clusterRef;
            return cfg;
        }
        case 'queue':
            return { ThroughputMaxMsgsPerSec: c.throughputMaxMsgsPerSec ?? 1000, WriteLatencyMs: c.writeLatencyMs ?? 2 };
        case 'cluster':
            return { MinReplicas: c.minReplicas ?? 1, MaxReplicas: c.maxReplicas ?? 5, HPA_Threshold: c.hpaThreshold ?? 0.7 };
        default:
            return { ...c };
    }
}

function buildSimulatePayload() {
    return {
        nodes: Object.values(state.nodes).map(n => ({
            id:     n.id,
            type:   n.type,
            label:  n.label,
            x:      n.x,
            y:      n.y,
            config: serializeNodeConfig(n),
        })),
        edges: Object.values(state.edges).map(e => ({
            id:           e.id,
            from:         e.from,
            to:           e.to,
            type:         e.type,
            trafficShare: e.trafficShare,
            config:       e.config,
        })),
    };
}

function applySimulationResult(result) {
    // Armazena resultados nos nós e arestas do state
    result.nodes.forEach(nr => {
        if (state.nodes[nr.id]) state.nodes[nr.id]._simResult = nr;
    });
    result.edges.forEach(er => {
        if (state.edges[er.id]) state.edges[er.id]._simResult = er;
    });

    // Re-renderiza tudo com dados de simulação
    Object.values(state.nodes).forEach(renderNode);
    renderEdges();

    // Exibe legenda
    document.getElementById('canvas-legend')?.classList.add('visible');

    // Atualiza painel se houver nó selecionado
    if (state.selectedId && state.selectedType === 'node') showProperties(state.selectedId, 'node');

    // Exibe painel de gargalos (e oferece acesso às rotas)
    showBottleneckDashboard(result);
}

function showBottleneckDashboard(result) {
    document.getElementById('bottleneck-modal')?.remove();

    const TOP_N = 5;

    // Exclui client e cluster (não têm utilização significativa)
    const rankable = result.nodes.filter(nr => {
        const type = state.nodes[nr.id]?.type;
        return type && type !== 'client' && type !== 'cluster';
    });

    const topNodes = [...rankable]
        .sort((a, b) => b.utilization - a.utilization)
        .slice(0, TOP_N);

    // Tráfego descartado: gateway drops + queue lag
    const dropDetails = [];
    let totalGatewayDrops = 0;
    let totalQueueLag    = 0;

    result.nodes.forEach(nr => {
        const type  = state.nodes[nr.id]?.type;
        const label = state.nodes[nr.id]?.label || nr.id;
        if (type === 'gateway' && nr.utilization > 1) {
            // effectiveRPS = rateLimitRPS quando capped; drops = (util-1) * effectiveRPS
            const drops = (nr.utilization - 1) * nr.effectiveRPS;
            totalGatewayDrops += drops;
            dropDetails.push({ label, kind: 'Gateway', value: drops, unit: 'RPS descartados' });
        }
        if (type === 'queue' && nr.metrics?.lagEst > 0) {
            totalQueueLag += nr.metrics.lagEst;
            dropDetails.push({ label, kind: 'Queue', value: nr.metrics.lagEst, unit: 'msg/s de lag' });
        }
    });

    // ─── Linhas da tabela de gargalos ─────────────────────────────────────────
    const bottleneckRows = topNodes.length === 0
        ? `<tr><td colspan="5" class="bn-empty">Nenhum componente analisável detectado.</td></tr>`
        : topNodes.map(nr => {
            const label      = state.nodes[nr.id]?.label || nr.id;
            const typeStr    = NODE_LABELS[state.nodes[nr.id]?.type] || (state.nodes[nr.id]?.type || '?');
            const icon       = nr.status === 'CRITICAL' ? '🔴' : nr.status === 'ALERT' ? '⚠️' : '🟢';
            const pct        = (nr.utilization * 100).toFixed(1);
            const statColor  = nr.status === 'CRITICAL' ? '#f38ba8'
                             : nr.status === 'ALERT'    ? '#f9e2af'
                             : '#a6e3a1';
            return `<tr>
                <td class="bn-icon">${icon}</td>
                <td class="bn-label">${label}</td>
                <td class="bn-type">${typeStr}</td>
                <td class="bn-util" style="color:${statColor};font-weight:700">${pct}%</td>
                <td class="bn-rps">${nr.effectiveRPS.toFixed(1)}</td>
            </tr>`;
        }).join('');

    // ─── Seção de tráfego descartado ──────────────────────────────────────────
    let dropHTML = '';
    if (dropDetails.length === 0) {
        dropHTML = `<p class="bn-no-drops">Nenhum tráfego descartado detectado.</p>`;
    } else {
        dropHTML = dropDetails.map(d => `
            <div class="bn-drop-entry">
                <span class="bn-drop-label">⚠️ ${d.label} <em>(${d.kind})</em></span>
                <span class="bn-drop-value">${d.value.toFixed(1)} ${d.unit}</span>
            </div>`).join('');
        if (totalGatewayDrops > 0)
            dropHTML += `<div class="bn-drop-total">Total descartado (Gateways): ${totalGatewayDrops.toFixed(1)} RPS</div>`;
        if (totalQueueLag > 0)
            dropHTML += `<div class="bn-drop-total">Total de lag (Queues): ${totalQueueLag.toFixed(1)} msg/s</div>`;
    }

    const hasRoutes = (result.routes?.length ?? 0) > 0;

    const modal = document.createElement('div');
    modal.id = 'bottleneck-modal';
    modal.className = 'bottleneck-modal';
    modal.innerHTML = `
        <div class="bottleneck-modal-content">
            <div class="bottleneck-modal-header">
                <span>Resumo da Simulação</span>
                <button class="bn-close-x">✕</button>
            </div>
            <div class="bottleneck-modal-body">
                <p class="bn-section-title">Top Gargalos (por utilização)</p>
                <table class="bn-table">
                    <thead>
                        <tr>
                            <th></th>
                            <th>Nó</th>
                            <th>Tipo</th>
                            <th>Utilização</th>
                            <th>RPS Efetivo</th>
                        </tr>
                    </thead>
                    <tbody>${bottleneckRows}</tbody>
                </table>
                <p class="bn-section-title" style="margin-top:16px">Tráfego Descartado / Lag</p>
                <div class="bn-drops">${dropHTML}</div>
            </div>
            <div class="bottleneck-modal-footer">
                ${hasRoutes ? `<button class="bn-btn-routes btn-secondary">Ver Métricas de Rotas</button>` : ''}
                <button class="bn-btn-close btn-primary">Fechar</button>
            </div>
        </div>
    `;

    document.body.appendChild(modal);

    const close = () => modal.remove();
    modal.querySelector('.bn-close-x').addEventListener('click', close);
    modal.querySelector('.bn-btn-close').addEventListener('click', close);
    modal.addEventListener('click', e => { if (e.target === modal) close(); });

    if (hasRoutes) {
        modal.querySelector('.bn-btn-routes').addEventListener('click', () => {
            close();
            showRoutesModal(result.routes);
        });
    }
}

function showRoutesModal(routes) {
    document.getElementById('routes-modal')?.remove();

    const modal = document.createElement('div');
    modal.id = 'routes-modal';
    modal.className = 'routes-modal';

    const nodeLabel = id => state.nodes[id]?.label || id;

    const rows = routes.map(r => `
        <tr>
            <td class="route-path">${r.path.map(nodeLabel).join(' → ')}</td>
            <td class="route-metric">${r.latencyMs.toFixed(1)} ms</td>
            <td class="route-metric">${r.p95Ms.toFixed(1)} ms</td>
            <td class="route-metric">${r.p99Ms.toFixed(1)} ms</td>
        </tr>
    `).join('');

    modal.innerHTML = `
        <div class="routes-modal-content">
            <div class="routes-modal-header">
                <span>Métricas de Latência por Rota</span>
                <button id="routes-modal-close">✕</button>
            </div>
            <table class="routes-table">
                <thead>
                    <tr>
                        <th>Rota</th>
                        <th>p50 (média)</th>
                        <th>p95 (×1.5)</th>
                        <th>p99 (×2.0)</th>
                    </tr>
                </thead>
                <tbody>${rows}</tbody>
            </table>
        </div>
    `;
    document.body.appendChild(modal);
    document.getElementById('routes-modal-close').addEventListener('click', () => modal.remove());
    modal.addEventListener('click', e => { if (e.target === modal) modal.remove(); });
}

document.getElementById('btn-simulate').addEventListener('click', async () => {
    const nodes = Object.values(state.nodes);
    if (nodes.length === 0) {
        setStatus('Canvas vazio — adicione componentes antes de simular.', 'error');
        return;
    }
    if (!nodes.some(n => n.type === 'client')) {
        setStatus('Adicione pelo menos um Client antes de simular.', 'error');
        return;
    }

    setStatus('Simulando…', 'warn');
    document.getElementById('btn-simulate').disabled = true;

    try {
        const res = await fetch('/simulate', {
            method:  'POST',
            headers: { 'Content-Type': 'application/json' },
            body:    JSON.stringify(buildSimulatePayload()),
        });
        const data = await res.json();
        if (!res.ok) {
            setStatus(`Erro: ${data.error}`, 'error');
            return;
        }
        applySimulationResult(data);
        setStatus(`Simulação concluída — ${data.routes?.length ?? 0} rota(s) analisada(s).`, 'success');
    } catch (err) {
        setStatus(`Falha na requisição: ${err.message}`, 'error');
    } finally {
        document.getElementById('btn-simulate').disabled = false;
    }
});

// ─── Export / Import JSON ─────────────────────────────────────────────────────

function exportJSON() {
    const payload = {
        name:  state.scenarioName,
        nodes: Object.values(state.nodes).map(n => ({
            id:     n.id,
            type:   n.type,
            label:  n.label,
            x:      n.x,
            y:      n.y,
            config: { ...n.config },
        })),
        edges: Object.values(state.edges).map(e => ({
            id:           e.id,
            from:         e.from,
            to:           e.to,
            type:         e.type,
            trafficShare: e.trafficShare,
            config:       { ...e.config },
        })),
    };

    const safeName = state.scenarioName.replace(/[^a-zA-Z0-9_\- ]/g, '').trim() || 'simulaarch';
    const filename = `${safeName}.json`;

    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' });
    const url  = URL.createObjectURL(blob);
    const a    = document.createElement('a');
    a.href     = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
    setStatus(`Diagrama exportado como ${filename}.`, 'success');
}

function importJSON(file) {
    if (!file) return;
    if (!file.name.endsWith('.json')) {
        setStatus('Erro: selecione um arquivo .json válido.', 'error');
        return;
    }

    const reader = new FileReader();
    reader.onload = e => {
        let data;
        try {
            data = JSON.parse(e.target.result);
        } catch {
            setStatus('Erro: arquivo JSON inválido ou malformado.', 'error');
            return;
        }

        // Validação mínima da estrutura
        if (!Array.isArray(data.nodes) || !Array.isArray(data.edges)) {
            setStatus('Erro: JSON não contém "nodes" e "edges" válidos.', 'error');
            return;
        }
        for (const n of data.nodes) {
            if (!n.id || !n.type) {
                setStatus('Erro: nó sem "id" ou "type" no JSON importado.', 'error');
                return;
            }
        }
        for (const ed of data.edges) {
            if (!ed.id || !ed.from || !ed.to) {
                setStatus('Erro: aresta sem "id", "from" ou "to" no JSON importado.', 'error');
                return;
            }
        }

        // Restaura state (sem resultado de simulação anterior)
        Object.keys(state.nodes).forEach(k => delete state.nodes[k]);
        Object.keys(state.edges).forEach(k => delete state.edges[k]);

        data.nodes.forEach(n => {
            state.nodes[n.id] = {
                id:     n.id,
                type:   n.type,
                label:  n.label  || NODE_LABELS[n.type] || n.type,
                x:      n.x      ?? 100,
                y:      n.y      ?? 100,
                config: n.config  || getDefaultConfig(n.type),
            };
        });
        data.edges.forEach(ed => {
            state.edges[ed.id] = {
                id:           ed.id,
                from:         ed.from,
                to:           ed.to,
                type:         ed.type         || 'sync',
                trafficShare: ed.trafficShare  ?? 1.0,
                config:       ed.config        || {},
            };
        });

        if (data.name) setScenarioName(data.name);

        deselectAll();
        setStatus(`Importado: ${data.nodes.length} nó(s), ${data.edges.length} conexão(ões).`, 'success');
    };
    reader.readAsText(file);
}

document.getElementById('btn-export').addEventListener('click', exportJSON);

document.getElementById('btn-import').addEventListener('click', () => {
    document.getElementById('file-input').click();
});

document.getElementById('file-input').addEventListener('change', e => {
    importJSON(e.target.files[0]);
    e.target.value = ''; // reset para permitir reimportar o mesmo arquivo
});

// ─── Gestão de Cenário ────────────────────────────────────────────────────────

function setScenarioName(name) {
    state.scenarioName = name || 'Meu Cenário';
    const el = document.getElementById('scenario-name');
    if (el) el.textContent = state.scenarioName;
    document.title = `${state.scenarioName} — SimulaArch`;
}

(function initScenarioTitle() {
    const el = document.getElementById('scenario-name');
    if (!el) return;

    let originalText = '';

    el.addEventListener('focus', () => {
        originalText = el.textContent;
        // Select all text on focus
        const range = document.createRange();
        range.selectNodeContents(el);
        const sel = window.getSelection();
        sel.removeAllRanges();
        sel.addRange(range);
    });

    el.addEventListener('keydown', e => {
        if (e.key === 'Enter') {
            e.preventDefault();
            el.blur();
        }
        if (e.key === 'Escape') {
            el.textContent = originalText;
            el.blur();
        }
    });

    el.addEventListener('blur', () => {
        const trimmed = el.textContent.trim();
        if (!trimmed) {
            el.textContent = originalText || 'Meu Cenário';
        }
        setScenarioName(el.textContent.trim());
        setStatus(`Cenário renomeado para "${state.scenarioName}".`, 'success');
    });

    // Prevent paste from injecting HTML
    el.addEventListener('paste', e => {
        e.preventDefault();
        const text = e.clipboardData.getData('text/plain');
        const sel = window.getSelection();
        if (!sel.rangeCount) return;
        sel.deleteFromDocument();
        sel.getRangeAt(0).insertNode(document.createTextNode(text));
        sel.collapseToEnd();
    });
})();

// ─── Init ─────────────────────────────────────────────────────────────────────

setStatus('Pronto — arraste componentes da paleta para o canvas.');