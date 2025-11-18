const express = require('express');

const app = express();
const PORT = 9999;

// Middleware
app.use(express.json());
app.use((req, res, next) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type');
  if (req.method === 'OPTIONS') {
    return res.sendStatus(200);
  }
  next();
});

// Mock data
const mockFileTree = {
  default: {
    metadata: {
      path: '',
      name: 'default',
      type: 'directory',
      is_directory: true,
      size: 0,
      modified_time: new Date().toISOString(),
      is_markdown: false,
      has_children: true,
      child_count: 2,
    },
    children: [
      {
        metadata: {
          path: 'notes',
          name: 'notes',
          type: 'directory',
          is_directory: true,
          size: 0,
          modified_time: new Date().toISOString(),
          is_markdown: false,
          has_children: true,
          child_count: 2,
        },
        children: [
          {
            metadata: {
              path: 'notes/todo.md',
              name: 'todo.md',
              type: 'file',
              is_directory: false,
              size: 156,
              modified_time: new Date().toISOString(),
              is_markdown: true,
              has_children: false,
              child_count: 0,
            },
            children: [],
            loaded: true,
          },
          {
            metadata: {
              path: 'notes/work.md',
              name: 'work.md',
              type: 'file',
              is_directory: false,
              size: 234,
              modified_time: new Date().toISOString(),
              is_markdown: true,
              has_children: false,
              child_count: 0,
            },
            children: [],
            loaded: true,
          },
        ],
        loaded: true,
      },
      {
        metadata: {
          path: 'readme.md',
          name: 'readme.md',
          type: 'file',
          is_directory: false,
          size: 89,
          modified_time: new Date().toISOString(),
          is_markdown: true,
          has_children: false,
          child_count: 0,
        },
        children: [],
        loaded: true,
      },
    ],
    loaded: true,
  },
};

const mockAppJs = `
const app = document.getElementById('app');
const path = window.location.hash.slice(1) || '/';

if (path.startsWith('/vault/')) {
  const vaultId = path.split('/')[2];
  renderVaultView(vaultId);
} else {
  renderHome();
}

function renderHome() {
  app.innerHTML = '<div style="padding: 2rem;"><h1>Obsidian Web - Mock</h1><p>Navigate to /#/vault/default</p></div>';
}

function renderVaultView(vaultId) {
  app.innerHTML = \`
    <div class="vault-view">
      <aside class="sidebar">
        <div class="vault-name">Vault \${vaultId}</div>
        <div class="status-indicator connected">
          <span class="fa fa-circle" style="color: #98c379;"></span>
          <span class="status-text">Live</span>
        </div>
        <div class="file-tree">
          <ul class="file-tree-list" id="tree"></ul>
        </div>
      </aside>
      <main class="main-content">
        <p>Main content will be here.</p>
      </main>
    </div>
  \`;

  loadTree(vaultId);
}

function loadTree(vaultId) {
  fetch(\`/api/v1/files/tree/\${vaultId}\`)
    .then(r => r.json())
    .then(data => renderTree(data.data.children))
    .catch(e => console.error(e));
}

function renderTree(nodes) {
  const treeEl = document.getElementById('tree');
  if (!treeEl) return;
  treeEl.innerHTML = nodes.map(renderNode).join('');

  document.querySelectorAll('.node-header').forEach(header => {
    header.addEventListener('click', toggleNode);
  });
}

function toggleNode(e) {
  const childrenDiv = e.target.closest('.tree-node').querySelector('.children');
  const expandIcon = e.target.closest('.node-header').querySelector('.expand-icon');

  if (!childrenDiv) return;

  if (childrenDiv.style.display === 'none') {
    childrenDiv.style.display = 'block';
    expandIcon.textContent = '▼';
  } else {
    childrenDiv.style.display = 'none';
    expandIcon.textContent = '▶';
  }
}

function renderNode(node) {
  const isDir = node.metadata.is_directory;
  const icon = isDir ? '<i class="fa fa-folder"></i>' : '<i class="fa fa-file-alt"></i>';
  const expand = isDir ? '<span class="expand-icon">▶</span>' : '<span class="expand-icon"></span>';

  return \`
    <li class="tree-node">
      <div class="node-header">
        \${expand}
        <span class="icon">\${icon}</span>
        <span class="node-name">\${node.metadata.name}</span>
      </div>
      \${renderChildren(node.children || [])}
    </li>
  \`;
}

function renderChildren(children) {
  if (!children || children.length === 0) return '';
  return \`
    <div class="children">
      <ul class="file-tree-list">
        \${children.map(renderNode).join('')}
      </ul>
    </div>
  \`;
}

window.addEventListener('hashchange', () => {
  const path = window.location.hash.slice(1) || '/';
  if (path.startsWith('/vault/')) {
    const vaultId = path.split('/')[2];
    renderVaultView(vaultId);
  } else {
    renderHome();
  }
});
`;

function getHtmlPage() {
  return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Obsidian Web - Mock</title>
  <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css">
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
    #app { min-height: 100vh; }
    .vault-view { display: flex; height: 100vh; }
    .sidebar { width: 250px; padding: 1rem; border-right: 1px solid #e0e0e0; background: #f5f5f5; overflow-y: auto; }
    .main-content { flex: 1; padding: 2rem; }
    .vault-name { font-size: 1.2rem; font-weight: bold; color: #1976d2; margin-bottom: 0.5rem; }
    .status-indicator { display: inline-flex; align-items: center; gap: 0.5rem; font-size: 0.85rem; margin-top: 0.5rem; }
    .status-indicator.connected { color: #98c379; }
    .status-text { color: #666; }
    .file-tree { margin-top: 1rem; }
    .file-tree-list { list-style: none; }
    .tree-node { margin: 0; padding: 0; }
    .node-header { display: flex; align-items: center; padding: 0.3rem 0.5rem; cursor: pointer; border-radius: 4px; user-select: none; }
    .node-header:hover { background: rgba(0,0,0,0.05); }
    .expand-icon { display: inline-flex; width: 16px; margin-right: 4px; font-size: 0.7rem; }
    .icon { display: inline-flex; width: 18px; height: 18px; margin-right: 6px; }
    .fa-folder { color: #f0c674; }
    .fa-folder-open { color: #f0c674; }
    .fa-file-alt { color: #56b6c2; }
    .node-name { color: #333; }
    .children { margin-left: 20px; padding-left: 8px; border-left: 1px solid #e0e0e0; }
  </style>
</head>
<body>
  <div id="app"></div>
  <script>
${mockAppJs}
  </script>
</body>
</html>`;
}

// API Routes

// Health check
app.get('/api/v1/health', (req, res) => {
  res.json({
    data: {
      status: 'ok',
      timestamp: new Date().toISOString(),
      uptime: process.uptime(),
    },
  });
});

// File tree
app.get('/api/v1/files/tree/:vaultId', (req, res) => {
  const { vaultId } = req.params;
  if (!mockFileTree[vaultId]) {
    return res.status(404).json({ error: 'Vault not found' });
  }
  res.json({ data: mockFileTree[vaultId] });
});

// File children
app.get('/api/v1/files/children/:vaultId', (req, res) => {
  const { vaultId } = req.params;
  if (!mockFileTree[vaultId]) {
    return res.status(404).json({ error: 'Vault not found' });
  }
  res.json({ data: mockFileTree[vaultId].children || [] });
});

// File metadata
app.get('/api/v1/files/meta/:vaultId', (req, res) => {
  const { vaultId } = req.params;
  if (!mockFileTree[vaultId]) {
    return res.status(404).json({ error: 'Vault not found' });
  }
  res.json({ data: mockFileTree[vaultId].metadata });
});

// SSE endpoint
app.get('/api/v1/sse/:vaultId', (req, res) => {
  const { vaultId } = req.params;
  if (!mockFileTree[vaultId]) {
    return res.status(404).json({ error: 'Vault not found' });
  }

  res.setHeader('Content-Type', 'text/event-stream');
  res.setHeader('Cache-Control', 'no-cache');
  res.setHeader('Connection', 'keep-alive');

  const clientId = Math.random().toString(36).substring(7);
  res.write(`event: connected\ndata: {"client_id":"${clientId}","vault_id":"${vaultId}"}\n\n`);

  const pingInterval = setInterval(() => {
    res.write(`event: ping\ndata: {"type":"ping","timestamp":"${new Date().toISOString()}"}\n\n`);
  }, 30000);

  req.on('close', () => clearInterval(pingInterval));
});

// SPA fallback - serve HTML for all other routes
app.use((req, res) => {
  res.type('text/html').send(getHtmlPage());
});

// Start server
const server = app.listen(PORT, () => {
  console.log(`Mock server running on http://localhost:${PORT}`);
});

module.exports = server;
