// ACARS Message Review Application

const state = {
    messages: [],
    types: [],
    selectedId: null,
    offset: 0,
    limit: 50,
    filters: {
        type: '',
        hasMissing: false,
        search: '',
        goldenOnly: false
    }
};

// Initialise the application.
async function init() {
    await loadTypes();
    await loadStats();
    await loadMessages();
    setupEventListeners();
}

// Load parser types for filter dropdown.
async function loadTypes() {
    try {
        const response = await fetch('/api/types');
        state.types = await response.json();

        const select = document.getElementById('type-filter');
        state.types.forEach(type => {
            const option = document.createElement('option');
            option.value = type;
            option.textContent = type;
            select.appendChild(option);
        });
    } catch (err) {
        console.error('Failed to load types:', err);
    }
}

// Load statistics.
async function loadStats() {
    try {
        const response = await fetch('/api/stats');
        const stats = await response.json();

        document.getElementById('stats').textContent =
            `${stats.TotalMessages} messages, ${stats.WithMissing} with missing fields`;
    } catch (err) {
        console.error('Failed to load stats:', err);
    }
}

// Load messages with current filters.
async function loadMessages() {
    const list = document.getElementById('message-list');
    list.innerHTML = '<div class="loading">Loading messages...</div>';

    try {
        const params = new URLSearchParams();
        if (state.filters.type) params.set('type', state.filters.type);
        if (state.filters.hasMissing) params.set('has_missing', 'true');
        if (state.filters.search) params.set('search', state.filters.search);
        if (state.filters.goldenOnly) params.set('golden', 'true');
        params.set('limit', state.limit);
        params.set('offset', state.offset);
        params.set('order', 'id');
        params.set('desc', 'true');

        const response = await fetch(`/api/messages?${params}`);
        state.messages = await response.json() || [];

        renderMessages();
        renderPagination();
    } catch (err) {
        console.error('Failed to load messages:', err);
        list.innerHTML = '<div class="loading">Error loading messages</div>';
    }
}

// Render the message list.
function renderMessages() {
    const list = document.getElementById('message-list');

    if (state.messages.length === 0) {
        list.innerHTML = '<div class="loading">No messages found</div>';
        return;
    }

    list.innerHTML = state.messages.map(msg => {
        const route = [msg.origin, msg.destination].filter(Boolean).join(' → ') || 'Unknown route';
        const confidence = (msg.confidence * 100).toFixed(0);
        const confidenceClass = confidence < 50 ? 'low' : confidence < 75 ? 'medium' : '';
        const goldenClass = msg.is_golden ? 'golden' : '';
        const selectedClass = msg.id === state.selectedId ? 'selected' : '';

        return `
            <div class="message-card ${goldenClass} ${selectedClass}" data-id="${msg.id}">
                <div class="message-header">
                    <span class="message-id">#${msg.id}</span>
                    <span class="message-type">${msg.parser_type}</span>
                </div>
                <div class="message-route">${escapeHtml(route)}</div>
                <div class="message-meta">
                    <span>${msg.flight || 'No flight'}</span>
                    <span>${msg.tail || 'No tail'}</span>
                    <span class="confidence ${confidenceClass}">${confidence}% confidence</span>
                    ${msg.missing_fields && msg.missing_fields.length ?
                        `<span class="missing-fields">${msg.missing_fields.length} missing</span>` : ''}
                </div>
            </div>
        `;
    }).join('');

    // Add click handlers.
    list.querySelectorAll('.message-card').forEach(card => {
        card.addEventListener('click', () => {
            const id = parseInt(card.dataset.id);
            selectMessage(id);
        });
    });
}

// Render pagination controls.
function renderPagination() {
    const pagination = document.getElementById('pagination');
    const hasPrev = state.offset > 0;
    const hasNext = state.messages.length === state.limit;

    pagination.innerHTML = `
        <button ${hasPrev ? '' : 'disabled'} id="prev-page">Previous</button>
        <span>Page ${Math.floor(state.offset / state.limit) + 1}</span>
        <button ${hasNext ? '' : 'disabled'} id="next-page">Next</button>
    `;

    document.getElementById('prev-page')?.addEventListener('click', () => {
        state.offset = Math.max(0, state.offset - state.limit);
        loadMessages();
    });

    document.getElementById('next-page')?.addEventListener('click', () => {
        state.offset += state.limit;
        loadMessages();
    });
}

// Select and show message details.
async function selectMessage(id) {
    state.selectedId = id;

    // Update selection in list.
    document.querySelectorAll('.message-card').forEach(card => {
        card.classList.toggle('selected', parseInt(card.dataset.id) === id);
    });

    // Load full message details.
    try {
        const response = await fetch(`/api/messages/${id}`);
        const msg = await response.json();
        renderDetail(msg);
        document.getElementById('detail-panel').classList.add('open');
    } catch (err) {
        console.error('Failed to load message:', err);
    }
}

// Render message detail panel.
function renderDetail(msg) {
    const content = document.getElementById('detail-content');

    const parsedFields = msg.parsed || {};
    const fieldsHtml = Object.entries(parsedFields)
        .filter(([k, v]) => !['message_id', 'timestamp', 'raw_text', 'parse_confidence'].includes(k))
        .map(([k, v]) => {
            const isEmpty = v === '' || v === null || v === undefined ||
                           (Array.isArray(v) && v.length === 0);
            const valueClass = isEmpty ? 'field-value missing' : 'field-value';
            const displayValue = isEmpty ? '(empty)' :
                                 Array.isArray(v) ? v.join(', ') : String(v);
            return `
                <div class="field-name">${escapeHtml(k)}</div>
                <div class="${valueClass}">${escapeHtml(displayValue)}</div>
            `;
        }).join('');

    content.innerHTML = `
        <div class="detail-section">
            <h3>Message Info</h3>
            <div class="parsed-fields">
                <div class="field-name">ID</div>
                <div class="field-value">${msg.id}</div>
                <div class="field-name">Type</div>
                <div class="field-value">${msg.parser_type}</div>
                <div class="field-name">Label</div>
                <div class="field-value">${msg.label}</div>
                <div class="field-name">Confidence</div>
                <div class="field-value">${(msg.confidence * 100).toFixed(1)}%</div>
            </div>
        </div>

        <div class="detail-section">
            <h3>Raw Text</h3>
            <div class="raw-text">${escapeHtml(msg.raw_text)}</div>
        </div>

        <div class="detail-section">
            <h3>Parsed Fields</h3>
            <div class="parsed-fields">${fieldsHtml || '<em>No fields parsed</em>'}</div>
        </div>

        ${msg.missing_fields && msg.missing_fields.length ? `
            <div class="detail-section">
                <h3>Missing Fields</h3>
                <div class="missing-fields">${msg.missing_fields.join(', ')}</div>
            </div>
        ` : ''}

        <div class="detail-section">
            <h3>Annotation</h3>
            <textarea id="annotation-input" placeholder="Add notes about this message...">${escapeHtml(msg.annotation || '')}</textarea>
            <button id="save-annotation" style="margin-top: 10px;">Save Annotation</button>
        </div>

        <div class="actions">
            <button id="toggle-golden" class="golden-btn ${msg.is_golden ? 'active' : ''}">
                ${msg.is_golden ? '★ Golden' : '☆ Mark Golden'}
            </button>
        </div>
    `;

    // Event handlers.
    document.getElementById('toggle-golden').addEventListener('click', () => toggleGolden(msg.id, !msg.is_golden));
    document.getElementById('save-annotation').addEventListener('click', () => {
        const annotation = document.getElementById('annotation-input').value;
        saveAnnotation(msg.id, annotation);
    });
}

// Toggle golden status.
async function toggleGolden(id, golden) {
    try {
        await fetch(`/api/messages/${id}/golden`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ golden })
        });

        // Update local state and re-render.
        const msg = state.messages.find(m => m.id === id);
        if (msg) msg.is_golden = golden;

        renderMessages();
        selectMessage(id); // Refresh detail
    } catch (err) {
        console.error('Failed to toggle golden:', err);
    }
}

// Save annotation.
async function saveAnnotation(id, annotation) {
    try {
        await fetch(`/api/messages/${id}/annotation`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ annotation })
        });
        alert('Annotation saved');
    } catch (err) {
        console.error('Failed to save annotation:', err);
    }
}

// Set up event listeners.
function setupEventListeners() {
    // Type filter.
    document.getElementById('type-filter').addEventListener('change', (e) => {
        state.filters.type = e.target.value;
        state.offset = 0;
        loadMessages();
    });

    // Missing filter.
    document.getElementById('missing-filter').addEventListener('change', (e) => {
        state.filters.hasMissing = e.target.value === 'has_missing';
        state.offset = 0;
        loadMessages();
    });

    // Search.
    let searchTimeout;
    document.getElementById('search').addEventListener('input', (e) => {
        clearTimeout(searchTimeout);
        searchTimeout = setTimeout(() => {
            state.filters.search = e.target.value;
            state.offset = 0;
            loadMessages();
        }, 300);
    });

    // Golden filter.
    document.getElementById('golden-filter').addEventListener('click', (e) => {
        state.filters.goldenOnly = !state.filters.goldenOnly;
        e.target.classList.toggle('active', state.filters.goldenOnly);
        state.offset = 0;
        loadMessages();
    });

    // Refresh.
    document.getElementById('refresh').addEventListener('click', () => {
        loadStats();
        loadMessages();
    });

    // Close detail panel.
    document.getElementById('detail-close').addEventListener('click', () => {
        document.getElementById('detail-panel').classList.remove('open');
        state.selectedId = null;
        renderMessages();
    });

    // Keyboard shortcuts.
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') {
            document.getElementById('detail-panel').classList.remove('open');
            state.selectedId = null;
            renderMessages();
        }
    });

    // Export buttons.
    document.getElementById('export-json').addEventListener('click', () => {
        window.location.href = '/api/export/json';
    });

    document.getElementById('export-go').addEventListener('click', () => {
        window.location.href = '/api/export/go';
    });
}

// HTML escape helper.
function escapeHtml(text) {
    if (text === null || text === undefined) return '';
    const div = document.createElement('div');
    div.textContent = String(text);
    return div.innerHTML;
}

// Start the app.
init();
