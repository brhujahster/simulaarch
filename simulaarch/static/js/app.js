const state = {
    nodes: {},
    edges: {},
    selectedId: null,
    selectedType: null,
};

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

function generateId() {
    return 'node-' + Date.now() + '-' + Math.floor(Math.random() * 10000);
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

// ─── Render ───────────────────────────────────────────────────────────────────

function renderNode(node) {
    const layer = document.getElementById('nodes-layer');
    const existing = document.getElementById(node.id);
    if (existing) existing.remove();

    const color = NODE_COLORS[node.type] || '#cdd6f4';
    const isSelected = state.selectedId === node.id;

    const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    g.setAttribute('id', node.id);
    g.setAttribute('class', 'canvas-node');
    g.setAttribute('transform', `translate(${node.x}, ${node.y})`);
    g.style.cursor = 'pointer';

    g.innerHTML = `
        <rect class="node-bg" x="0" y="0" width="${NODE_W}" height="${NODE_H}"
              rx="8" fill="#1e1e2e"
              stroke="${isSelected ? '#f9e2af' : color}"
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
                style="cursor:crosshair" data-id="${node.id}"/>
    `;

    let dragging = false;
    let dragOffX = 0, dragOffY = 0;

    g.addEventListener('mousedown', e => {
        if (e.target.classList.contains('node-port')) return;
        e.stopPropagation();

        selectNode(node.id);

        const svg = document.getElementById('canvas');
        const rect = svg.getBoundingClientRect();
        dragOffX = e.clientX - rect.left - node.x;
        dragOffY = e.clientY - rect.top - node.y;
        dragging = true;

        const onMove = ev => {
            if (!dragging) return;
            node.x = ev.clientX - rect.left - dragOffX;
            node.y = ev.clientY - rect.top - dragOffY;
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

function renderEdges() {
    const layer = document.getElementById('edges-layer');
    layer.innerHTML = '';

    Object.values(state.edges).forEach(edge => {
        const from = state.nodes[edge.from];
        const to   = state.nodes[edge.to];
        if (!from || !to) return;

        const x1 = from.x + NODE_W;
        const y1 = from.y + NODE_H / 2;
        const x2 = to.x;
        const y2 = to.y + NODE_H / 2;
        const mx = (x1 + x2) / 2;

        const isAsync = edge.type === 'async';
        const color   = isAsync ? '#fab387' : '#585b70';
        const marker  = isAsync ? 'url(#arrowhead-async)' : 'url(#arrowhead)';

        const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
        path.setAttribute('d', `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`);
        path.setAttribute('fill', 'none');
        path.setAttribute('stroke', color);
        path.setAttribute('stroke-width', '2');
        if (isAsync) path.setAttribute('stroke-dasharray', '6,3');
        path.setAttribute('marker-end', marker);
        path.style.cursor = 'pointer';
        path.dataset.id = edge.id;

        path.addEventListener('click', e => {
            e.stopPropagation();
            selectNode(edge.id, 'edge');
        });

        layer.appendChild(path);
    });
}

function renderAll() {
    Object.values(state.nodes).forEach(renderNode);
    renderEdges();
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
    renderAll();
    document.getElementById('properties-panel').innerHTML =
        '<p id="properties-placeholder">Selecione um componente</p>';
}

// ─── Properties Panel ─────────────────────────────────────────────────────────

function showProperties(id, type) {
    const panel = document.getElementById('properties-panel');
    if (type === 'node') {
        const node = state.nodes[id];
        if (!node) return;
        panel.innerHTML = `
            <p class="prop-section-title">Propriedades</p>
            <label class="prop-label">Nome</label>
            <input id="prop-label" type="text" class="prop-input" value="${node.label}"/>
            <p class="prop-type-badge">Tipo: <strong>${NODE_LABELS[node.type]}</strong></p>
            <hr class="prop-divider"/>
            <button id="btn-delete-node" class="btn-danger">Remover nó</button>
        `;
        document.getElementById('prop-label').addEventListener('input', e => {
            node.label = e.target.value;
            renderNode(node);
        });
        document.getElementById('btn-delete-node').addEventListener('click', () => deleteSelected());
    }
}

// ─── Delete ───────────────────────────────────────────────────────────────────

function deleteSelected() {
    if (!state.selectedId) return;

    if (state.selectedType === 'node') {
        const id = state.selectedId;
        delete state.nodes[id];
        Object.keys(state.edges).forEach(eid => {
            if (state.edges[eid].from === id || state.edges[eid].to === id)
                delete state.edges[eid];
        });
        setStatus('Nó removido.', 'warn');
    } else if (state.selectedType === 'edge') {
        delete state.edges[state.selectedId];
        setStatus('Conexão removida.', 'warn');
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
    item.addEventListener('dragend', () => {
        item.classList.remove('dragging');
    });
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

        const rect = svgCanvas.getBoundingClientRect();
        const x = e.clientX - rect.left - NODE_W / 2;
        const y = e.clientY - rect.top  - NODE_H / 2;
        const id = generateId();

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

// ─── Click canvas background = deselect ──────────────────────────────────────

svgCanvas.addEventListener('click', e => {
    if (e.target === svgCanvas) deselectAll();
});

// ─── Keyboard: Delete ─────────────────────────────────────────────────────────

document.addEventListener('keydown', e => {
    if ((e.key === 'Delete' || e.key === 'Backspace') &&
        document.activeElement.tagName !== 'INPUT') {
        deleteSelected();
    }
});

// ─── Toolbar buttons ──────────────────────────────────────────────────────────

document.getElementById('btn-new').addEventListener('click', () => {
    if (Object.keys(state.nodes).length === 0) return;
    if (!confirm('Criar novo diagrama? O atual será perdido.')) return;
    Object.keys(state.nodes).forEach(k => delete state.nodes[k]);
    Object.keys(state.edges).forEach(k => delete state.edges[k]);
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

document.getElementById('btn-simulate').addEventListener('click', () => {
    setStatus('Simulação ainda não implementada.', 'warn');
});

document.getElementById('btn-export').addEventListener('click', () => {
    setStatus('Exportação ainda não implementada.', 'warn');
});

document.getElementById('btn-import').addEventListener('click', () => {
    document.getElementById('file-input').click();
});

// ─── Init ─────────────────────────────────────────────────────────────────────

setStatus('Pronto — arraste componentes da paleta para o canvas.');