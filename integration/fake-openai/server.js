const http = require('http');

const server = http.createServer(async (req, res) => {
  if (req.url === '/health') {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ ok: true }));
    return;
  }

  if (req.method === 'POST' && req.url === '/embeddings') {
    let body = '';
    req.on('data', chunk => body += chunk);
    req.on('end', () => {
      try {
        const parsed = JSON.parse(body || '{}');
        const input = parsed.input || '';
        const dim = 8;
        const vector = Array(dim).fill(0).map((_, idx) => (idx + (input.length % (idx + 1))) / dim);
        const response = {
          model: parsed.model || 'fake-embedding-model',
          data: [
            { embedding: vector }
          ]
        };
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(response));
      } catch (err) {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'invalid request', detail: err.message }));
      }
    });
    return;
  }

  res.writeHead(404, { 'Content-Type': 'application/json' });
  res.end(JSON.stringify({ error: 'not found' }));
});

const port = process.env.PORT || 8081;
server.listen(port, () => {
  console.log(`fake-openai listening on ${port}`);
});
