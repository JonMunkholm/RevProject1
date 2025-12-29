# RevProject Documentation Overview

This directory contains all governance, architecture, and operational documentation for the **Embedding Governance & Retrieval Architecture (EGRA)** system.

---

## 📘 Core Documents

| File                                          | Purpose                                                                                | Status         |
| --------------------------------------------- | -------------------------------------------------------------------------------------- | -------------- |
| **embedding_system_master_blueprint_v1.0.md** | Canonical architecture, schema, and governance specification for the embedding system. | ✅ Active      |
| **DECISIONS.md**                              | Log of approved design and governance decisions (Change Classes CC-1..CC-3).           | ✅ Active      |
| **adr/**                                      | Individual Architecture Decision Records (one per major choice).                       | ✅ Active      |
| **gold_tests.md**                             | Description of test coverage, recall targets, and drift thresholds.                    | 🧩 Planned     |
| **ops/**                                      | Operational reports, restore logs, and checksum audits.                                | ✅ Active      |

---

## 🧭 Versioning & Naming Policy

Blueprints follow a **semantic versioning convention** aligned with schema and governance changes:

| Version Type | File Example                                  | Trigger                                                |
| ------------ | --------------------------------------------- | ------------------------------------------------------ |
| **Major**    | `embedding_system_master_blueprint_v2.0.md`   | Major schema change or governance overhaul             |
| **Minor**    | `embedding_system_master_blueprint_v1.1.md`   | Added optional sections, metrics, or pipeline features |
| **Patch**    | `embedding_system_master_blueprint_v1.0.1.md` | Typos, editorial, or small corrections                 |

All previous versions remain under `/docs/` for historical and audit reference.

---

## 🧩 CI Integration

A simple check ensures that the schema and documentation remain in sync:

```bash
# CI pseudo-check
if grep -q "SCHEMA_VERSION=v1.0-2025-10-15" .env.example; then
  echo "✅ Schema version matches blueprint v1.0"
else
  echo "❌ Schema mismatch: update blueprint or migrations"
  exit 1
fi
```
