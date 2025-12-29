# Stage 1 — Working App Checklist

**Goal:** Implement the minimal end-to-end slice of the Embedding Governance & Retrieval Architecture (EGRA) so that a document can be ingested, embedded, stored, and retrieved successfully.

> _Upload / ingest → store → embed → retrieve → answer._

_2025-10-18: Verified ingestion (`go run ./cmd/ingest -file docs/sample_paragraph.txt`) plus retrieval (`curl http://localhost:8080/api/search?q=performance+obligations`)._

---

## 🧱 1. Environment & Setup

| Task                          | Description                                                                                          | Status |
| :---------------------------- | :--------------------------------------------------------------------------------------------------- | :----: |
| **Create project skeleton**   | Folders: `cmd/ingest`, `cmd/api`, `internal/database`, `internal/retrieval`, `internal/ai`, `docs/`. |   ✅    |
| **Initialize Go module**      | `go mod init github.com/yourorg/revproject1`                                                         |   ✅    |
| **Install dependencies**      | `pgx/v5`, `uuid`, `godotenv`, `chi` (or `gin`), `pgvector`.                                          |   ✅    |
| **Create .env file**          | Include `DB_URL`, `OPENAI_API_KEY`, `SCHEMA_VERSION=v1.0-2025-10-15`.                                |   ✅    |
| **Start Postgres + pgvector** | Docker Compose or local install; confirm `CREATE EXTENSION vector;`.                                 |   ✅    |
| **Apply schema**              | Run `goose -dir sql/schema up` (or rely on Stage 1 bootstrap via `cmd/ingest`) to create tables.     |   ✅    |

---

## 🧩 2. Ingestion Path (`/cmd/ingest`)

| Task                           | Description                                                                 | Status |
| :----------------------------- | :-------------------------------------------------------------------------- | :----: |
| **Parse local file**           | Read a `.txt` file containing an ASC 606 paragraph or policy excerpt.       |   ✅    |
| **Compute fingerprint**        | Generate SHA256 hash of content → store as `source_id`.                     |   ✅    |
| **Insert into DB**             | Write record to `asc_paragraphs` (`framework='US_GAAP'`, `topic='ASC606'`). |   ✅    |
| **Call embedding API**         | Use OpenAI `text-embedding-3-large`; store vector in `asc_embeddings`.      |   ✅    |
| **Console output**             | Print paragraph ID + embedding dimension for sanity check.                  |   ✅    |
| **Skip re-embed if unchanged** | Check existing `source_id`; bypass insert if duplicate.                     |   ✅    |

---

## 🔍 3. Retrieval Path (`/cmd/api/search`)

| Task                   | Description                                                                        | Status |
| :--------------------- | :--------------------------------------------------------------------------------- | :----: |
| **HTTP API**           | Expose `GET /api/search?q=...`.                                                    |   ✅    |
| **Embed query**        | Generate embedding for query text (using same model).                              |   ✅    |
| **Search DB**          | `SELECT *, embedding <-> $q AS score FROM asc_embeddings ORDER BY score LIMIT 5;`. |   ✅    |
| **Join metadata**      | Return `asc_reference`, `content`, `score`.                                        |   ✅    |
| **Return JSON**        | `{ "results": [{ "ref":"ASC606-10-25-1","score":0.87,"excerpt":"..."}] }`.         |   ✅    |
| **Serve on port 8080** | Run `go run cmd/api/main.go`; visit `http://localhost:8080/api/search?q=revenue`.  |   ✅    |

---

## 🧠 4. Sanity Tests

| Check                                                      | Expected Outcome |
| :--------------------------------------------------------- | :--------------- |
| Insert & query run without errors.                         | ✅               |
| Query “performance obligations” returns correct paragraph. | ✅               |
| Re-running ingest with same text skips duplicate insert.   | ✅               |
| Retrieval returns ≤ 5 results ordered by relevance.        | ✅               |
| End-to-end latency < 2 seconds for single query.           | ✅               |

---

## 🧰 5. Developer UX Enhancements (Optional)

| Task                  | Description                                         | Benefit                |
| :-------------------- | :-------------------------------------------------- | :--------------------- |
| **Simple HTML form**  | HTMX or bare HTML posting to `/api/search`.         | Quick demo UI.         |
| **Log retrievals**    | Insert record into `retrieval_log` for each search. | Replay testing.        |
| **Dockerfile**        | Containerize Postgres + API for easy spin-up.       | Portability.           |
| **Taskfile/Makefile** | Tasks: `ingest`, `run`, `search`.                   | Developer consistency. |
| **AI quota alerts**   | Surface embedding quota errors with logging / retry. | Smoother troubleshooting. |

---

## 🔐 6. Deferred for Later Phases

_You can skip these until after a working demo:_

- Multi-tenant roles & invitations
- S3 storage abstraction
- Worker queue / retry logic
- Rate limits or quotas
- Prometheus metrics & alerts
- Automated Decision Log integration
- Migrate database access from `database/sql` + `lib/pq` to `pgx/v5` once advanced driver features are required.
- Add pgx `COPY`-based bulk ingestion path to speed up future high-volume document loads.
- Leverage pgx context-aware timeouts and cancellation to improve long-running query handling.
- Capture pgx-specific query telemetry (latency, retries) once the driver migration ships.

---

## ✅ 7. Completion Criteria

You have:

1. A local Postgres + pgvector database.
2. A Go CLI (`/cmd/ingest`) that inserts and embeds text.
3. A Go API (`/cmd/api/search`) that retrieves and returns ranked paragraphs.
4. One working ASC 606 paragraph retrievable by keyword.
5. Logs showing successful end-to-end execution.

At that point, you have a **fully functional finance-grade AI prototype** — the foundation for Stage 2 (resilience + UI enhancements).
