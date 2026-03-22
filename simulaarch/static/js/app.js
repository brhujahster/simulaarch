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

function setStatus(msg, type = '') {
    const el = document.getElementById('status-text');
    el.textContent = msg;
    el.className = type;
}

document.querySelectorAll('.palette-item').forEach(item => {
    item.addEventListener('dragstart', e => {
        e.dataTransfer.setData('node-type', item.dataset.type);
        item.classList.add('dragging');
    });
    item.addEventListener('dragend', () => {
        item.classList.remove('dragging');
    });
});

setStatus('Pronto — arraste componentes da paleta para o canvas.');