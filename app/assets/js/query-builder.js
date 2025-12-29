"use strict";
/**
 * Query Builder Module
 *
 * Provides a rich SQL editing experience with:
 * - Monaco Editor for syntax highlighting and autocomplete
 * - Natural language to SQL generation via AI
 * - Query execution with results display
 * - CSV export functionality
 */
// ============================================================================
// State Management
// ============================================================================
class Store {
    constructor() {
        this.listeners = new Set();
        this.state = {
            sql: '',
            prompt: '',
            results: null,
            generationError: null,
            executionError: null,
            isGenerating: false,
            isExecuting: false,
            schema: null,
        };
    }
    getState() {
        return this.state;
    }
    setState(partial) {
        this.state = { ...this.state, ...partial };
        this.notify();
    }
    subscribe(listener) {
        this.listeners.add(listener);
        return () => this.listeners.delete(listener);
    }
    notify() {
        this.listeners.forEach(listener => listener(this.state));
    }
}
// ============================================================================
// API Client
// ============================================================================
class QueryAPI {
    async generateSQL(prompt) {
        const formData = new FormData();
        formData.append('prompt', prompt);
        const response = await fetch('/api/queries/generate', {
            method: 'POST',
            body: formData,
            credentials: 'include',
        });
        if (!response.ok) {
            // Try to parse as JSON error first
            const text = await response.text();
            try {
                const json = JSON.parse(text);
                if (json.error) {
                    throw new Error(json.error);
                }
            }
            catch (e) {
                // Not JSON, try to extract error from HTML
                if (e instanceof SyntaxError) {
                    throw new Error(this.extractError(text) || 'Failed to generate SQL');
                }
                throw e;
            }
            throw new Error('Failed to generate SQL');
        }
        // Success - return the SQL text
        return response.text();
    }
    async executeQuery(sql) {
        const formData = new FormData();
        formData.append('sql', sql);
        const response = await fetch('/api/queries/execute', {
            method: 'POST',
            body: formData,
            credentials: 'include',
        });
        const html = await response.text();
        if (!response.ok || html.includes('query-builder__results-error')) {
            throw new Error(this.extractError(html) || 'Query execution failed');
        }
        return this.parseResultsHTML(html);
    }
    downloadCSV(sql) {
        const form = document.createElement('form');
        form.method = 'POST';
        form.action = '/api/queries/download';
        const input = document.createElement('input');
        input.type = 'hidden';
        input.name = 'sql';
        input.value = sql;
        form.appendChild(input);
        document.body.appendChild(form);
        form.submit();
        document.body.removeChild(form);
    }
    extractError(html) {
        // Extract error message from HTML response
        const match = html.match(/<span>([^<]+)<\/span>/);
        return match ? match[1] : null;
    }
    parseResultsHTML(html) {
        const parser = new DOMParser();
        const doc = parser.parseFromString(html, 'text/html');
        const table = doc.querySelector('.query-builder__table');
        if (!table) {
            return { columns: [], rows: [], rowCount: 0 };
        }
        const columns = [];
        const headerCells = table.querySelectorAll('thead th');
        headerCells.forEach(th => columns.push(th.textContent || ''));
        const rows = [];
        const bodyRows = table.querySelectorAll('tbody tr');
        bodyRows.forEach(tr => {
            const row = [];
            const cells = tr.querySelectorAll('td');
            cells.forEach(td => row.push(td.textContent || ''));
            if (row.length > 0 && !tr.querySelector('.query-builder__table-empty')) {
                rows.push(row);
            }
        });
        return { columns, rows, rowCount: rows.length };
    }
}
// ============================================================================
// Monaco Editor Setup
// ============================================================================
class SQLEditor {
    constructor(container, onChange) {
        this.editor = null;
        this.container = container;
        this.onChange = onChange;
    }
    async initialize() {
        // Wait for Monaco to be available
        await this.waitForMonaco();
        this.editor = monaco.editor.create(this.container, {
            value: '',
            language: 'sql',
            theme: 'vs-dark',
            minimap: { enabled: false },
            fontSize: 14,
            fontFamily: "'JetBrains Mono', 'Fira Code', 'SF Mono', Consolas, monospace",
            lineNumbers: 'on',
            scrollBeyondLastLine: false,
            automaticLayout: true,
            tabSize: 2,
            wordWrap: 'on',
            padding: { top: 16, bottom: 16 },
            renderLineHighlight: 'line',
            cursorBlinking: 'smooth',
            smoothScrolling: true,
            contextmenu: true,
            folding: false,
            glyphMargin: false,
            lineDecorationsWidth: 8,
            lineNumbersMinChars: 3,
            overviewRulerBorder: false,
            scrollbar: {
                vertical: 'auto',
                horizontal: 'auto',
                verticalScrollbarSize: 10,
                horizontalScrollbarSize: 10,
            },
        });
        // Listen for changes
        this.editor.onDidChangeModelContent(() => {
            this.onChange(this.editor.getValue());
        });
        // Add keyboard shortcuts
        this.editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
            document.getElementById('run-query-btn')?.click();
        });
    }
    waitForMonaco() {
        return new Promise((resolve) => {
            if (typeof monaco !== 'undefined') {
                resolve();
                return;
            }
            const checkInterval = setInterval(() => {
                if (typeof monaco !== 'undefined') {
                    clearInterval(checkInterval);
                    resolve();
                }
            }, 50);
            // Timeout after 10 seconds
            setTimeout(() => {
                clearInterval(checkInterval);
                resolve();
            }, 10000);
        });
    }
    getValue() {
        return this.editor?.getValue() || '';
    }
    setValue(value) {
        if (this.editor && this.editor.getValue() !== value) {
            this.editor.setValue(value);
        }
    }
    focus() {
        this.editor?.focus();
    }
    registerCompletionProvider(schema) {
        if (!monaco)
            return;
        monaco.languages.registerCompletionItemProvider('sql', {
            provideCompletionItems: (model, position) => {
                const suggestions = [];
                // Add table names
                schema.forEach(table => {
                    suggestions.push({
                        label: table.name,
                        kind: monaco.languages.CompletionItemKind.Class,
                        insertText: table.name,
                        detail: 'Table',
                    });
                    // Add column names with table prefix
                    table.columns.forEach(col => {
                        suggestions.push({
                            label: `${table.name}.${col.name}`,
                            kind: monaco.languages.CompletionItemKind.Field,
                            insertText: `${table.name}.${col.name}`,
                            detail: `${col.type}${col.nullable ? '' : ' NOT NULL'}`,
                        });
                    });
                });
                // Add SQL keywords
                const keywords = [
                    'SELECT', 'FROM', 'WHERE', 'JOIN', 'LEFT JOIN', 'RIGHT JOIN', 'INNER JOIN',
                    'ON', 'AND', 'OR', 'NOT', 'IN', 'LIKE', 'BETWEEN', 'IS NULL', 'IS NOT NULL',
                    'ORDER BY', 'GROUP BY', 'HAVING', 'LIMIT', 'OFFSET', 'AS', 'DISTINCT',
                    'COUNT', 'SUM', 'AVG', 'MIN', 'MAX', 'CASE', 'WHEN', 'THEN', 'ELSE', 'END',
                ];
                keywords.forEach(kw => {
                    suggestions.push({
                        label: kw,
                        kind: monaco.languages.CompletionItemKind.Keyword,
                        insertText: kw,
                    });
                });
                return { suggestions };
            },
        });
    }
}
// ============================================================================
// UI Renderer
// ============================================================================
class UIRenderer {
    constructor() {
        this.elements = {
            promptInput: document.getElementById('nl-prompt'),
            generateBtn: document.getElementById('generate-sql-btn'),
            runBtn: document.getElementById('run-query-btn'),
            copyBtn: document.getElementById('copy-sql-btn'),
            clearBtn: document.getElementById('clear-sql-btn'),
            downloadBtn: document.getElementById('download-csv-btn'),
            resultsContainer: document.getElementById('query-results'),
            nlErrorContainer: document.getElementById('nl-error'),
            loadingIndicator: document.getElementById('loading-indicator'),
            nlLoadingIndicator: document.getElementById('nl-loading'),
        };
    }
    render(state) {
        this.renderLoading(state);
        this.renderGenerationError(state);
        this.renderResults(state);
        this.renderButtons(state);
    }
    renderLoading(state) {
        const indicator = this.elements.loadingIndicator;
        const nlIndicator = this.elements.nlLoadingIndicator;
        // Main loading overlay
        if (indicator) {
            if (state.isExecuting) {
                const textEl = indicator.querySelector('#loading-text');
                if (textEl)
                    textEl.textContent = 'Running query...';
                indicator.style.display = 'flex';
            }
            else {
                indicator.style.display = 'none';
            }
        }
        // NL section loading indicator
        if (nlIndicator) {
            nlIndicator.style.display = state.isGenerating ? 'flex' : 'none';
        }
    }
    renderGenerationError(state) {
        const container = this.elements.nlErrorContainer;
        if (!container)
            return;
        if (state.generationError) {
            container.innerHTML = `
        <div class="query-builder__nl-error">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="12" y1="8" x2="12" y2="12"></line>
            <line x1="12" y1="16" x2="12.01" y2="16"></line>
          </svg>
          <span>${this.escapeHtml(state.generationError)}</span>
        </div>
      `;
        }
        else {
            container.innerHTML = '';
        }
    }
    renderResults(state) {
        const container = this.elements.resultsContainer;
        if (!container)
            return;
        // Show execution error in results area
        if (state.executionError) {
            container.innerHTML = `
        <div class="query-builder__results-error">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="12" y1="8" x2="12" y2="12"></line>
            <line x1="12" y1="16" x2="12.01" y2="16"></line>
          </svg>
          <span>${this.escapeHtml(state.executionError)}</span>
        </div>
      `;
            return;
        }
        if (!state.results) {
            container.innerHTML = `
        <div class="query-builder__results-placeholder">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="query-builder__results-empty-icon">
            <path d="M3 3h18v18H3z"/>
            <path d="M3 9h18"/>
            <path d="M9 21V9"/>
          </svg>
          <p>Query results will appear here</p>
          <p class="query-builder__results-hint">Write SQL or use AI to generate a query, then click Run</p>
        </div>
      `;
            return;
        }
        const { columns, rows, rowCount } = state.results;
        if (columns.length === 0) {
            container.innerHTML = `
        <div class="query-builder__results-placeholder">
          <p>No results returned</p>
        </div>
      `;
            return;
        }
        const headerHtml = columns.map(col => `<th>${this.escapeHtml(col)}</th>`).join('');
        const rowsHtml = rows.length === 0
            ? `<tr><td colspan="${columns.length}" class="query-builder__table-empty">No results found</td></tr>`
            : rows.map(row => `<tr>${row.map(cell => `<td>${this.escapeHtml(cell)}</td>`).join('')}</tr>`).join('');
        container.innerHTML = `
      <div class="query-builder__results-content">
        <div class="query-builder__results-info">
          <span class="query-builder__row-count">${rowCount} row${rowCount !== 1 ? 's' : ''} returned</span>
        </div>
        <div class="query-builder__table-wrapper">
          <table class="query-builder__table">
            <thead><tr>${headerHtml}</tr></thead>
            <tbody>${rowsHtml}</tbody>
          </table>
        </div>
      </div>
    `;
    }
    renderButtons(state) {
        const { generateBtn, runBtn, downloadBtn } = this.elements;
        if (generateBtn) {
            generateBtn.disabled = state.isGenerating || state.isExecuting;
        }
        if (runBtn) {
            runBtn.disabled = state.isGenerating || state.isExecuting || !state.sql.trim();
        }
        if (downloadBtn) {
            downloadBtn.style.display = state.results && state.results.rows.length > 0 ? 'flex' : 'none';
        }
    }
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
    getPromptValue() {
        return this.elements.promptInput?.value.trim() || '';
    }
    clearPrompt() {
        if (this.elements.promptInput) {
            this.elements.promptInput.value = '';
        }
    }
    focusPrompt() {
        this.elements.promptInput?.focus();
    }
}
// ============================================================================
// Main Controller
// ============================================================================
class QueryBuilder {
    constructor() {
        this.editor = null;
        this.store = new Store();
        this.api = new QueryAPI();
        this.ui = new UIRenderer();
    }
    async initialize() {
        // Subscribe to state changes
        this.store.subscribe((state) => this.ui.render(state));
        // Initialize Monaco Editor
        const editorContainer = document.getElementById('sql-editor-container');
        if (editorContainer) {
            this.editor = new SQLEditor(editorContainer, (value) => {
                this.store.setState({ sql: value, executionError: null });
            });
            await this.editor.initialize();
        }
        // Bind event handlers
        this.bindEvents();
        // Initial render
        this.ui.render(this.store.getState());
    }
    bindEvents() {
        // Generate SQL button
        document.getElementById('generate-sql-btn')?.addEventListener('click', () => {
            this.generateSQL();
        });
        // Prompt input enter key
        document.getElementById('nl-prompt')?.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                this.generateSQL();
            }
        });
        // Run Query button
        document.getElementById('run-query-btn')?.addEventListener('click', () => {
            this.executeQuery();
        });
        // Copy button
        document.getElementById('copy-sql-btn')?.addEventListener('click', () => {
            this.copySQL();
        });
        // Clear button
        document.getElementById('clear-sql-btn')?.addEventListener('click', () => {
            this.clearSQL();
        });
        // Download CSV button
        document.getElementById('download-csv-btn')?.addEventListener('click', () => {
            this.downloadCSV();
        });
        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => {
            // Ctrl/Cmd + Enter to run query
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                const activeElement = document.activeElement;
                if (activeElement?.id === 'nl-prompt') {
                    this.generateSQL();
                }
                else {
                    this.executeQuery();
                }
            }
        });
    }
    async generateSQL() {
        const prompt = this.ui.getPromptValue();
        if (!prompt) {
            this.store.setState({ generationError: 'Please enter a query description' });
            this.ui.focusPrompt();
            return;
        }
        this.store.setState({ isGenerating: true, generationError: null });
        try {
            console.log('[QueryBuilder] Generating SQL for prompt:', prompt);
            const sql = await this.api.generateSQL(prompt);
            console.log('[QueryBuilder] Received SQL:', sql);
            // Clean up any HTML that might be in the response
            const cleanSQL = sql.replace(/<[^>]*>/g, '').trim();
            console.log('[QueryBuilder] Clean SQL:', cleanSQL);
            this.editor?.setValue(cleanSQL);
            this.store.setState({ sql: cleanSQL, isGenerating: false, generationError: null });
        }
        catch (err) {
            console.error('[QueryBuilder] Generation error:', err);
            this.store.setState({
                generationError: err instanceof Error ? err.message : 'Failed to generate SQL',
                isGenerating: false,
            });
        }
    }
    async executeQuery() {
        const sql = this.editor?.getValue() || this.store.getState().sql;
        if (!sql.trim()) {
            this.store.setState({ executionError: 'No SQL query to execute' });
            return;
        }
        this.store.setState({ isExecuting: true, executionError: null, results: null });
        try {
            const results = await this.api.executeQuery(sql);
            this.store.setState({ results, isExecuting: false, executionError: null });
        }
        catch (err) {
            this.store.setState({
                executionError: err instanceof Error ? err.message : 'Query execution failed',
                isExecuting: false,
            });
        }
    }
    copySQL() {
        const sql = this.editor?.getValue() || '';
        if (!sql)
            return;
        navigator.clipboard.writeText(sql).then(() => {
            const btn = document.getElementById('copy-sql-btn');
            if (btn) {
                const originalHTML = btn.innerHTML;
                btn.innerHTML = `
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="20 6 9 17 4 12"></polyline>
          </svg>
          Copied!
        `;
                setTimeout(() => {
                    btn.innerHTML = originalHTML;
                }, 2000);
            }
        });
    }
    clearSQL() {
        this.editor?.setValue('');
        this.store.setState({ sql: '', results: null, generationError: null, executionError: null });
        this.editor?.focus();
    }
    downloadCSV() {
        const sql = this.editor?.getValue() || '';
        if (!sql)
            return;
        this.api.downloadCSV(sql);
    }
}
// ============================================================================
// Initialize on DOM ready
// ============================================================================
function initQueryBuilder() {
    const queryBuilder = new QueryBuilder();
    queryBuilder.initialize().catch(console.error);
}
// Check if DOM is already loaded
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initQueryBuilder);
}
else {
    initQueryBuilder();
}
// Export for potential external use
window.QueryBuilder = QueryBuilder;
//# sourceMappingURL=query-builder.js.map