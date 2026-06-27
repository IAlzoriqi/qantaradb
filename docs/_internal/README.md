# QantaraDB Internal Development Documentation

Status: internal early-development draft  
Audience: core maintainers and implementation agents only  
Public marketing status: do not link from the public README, package docs, website, release notes, or repository homepage until the project reaches the documented public-readiness gates.

## Purpose

This directory records the private development roadmap for turning QantaraDB from a focused MySQL/MariaDB to PostgreSQL migration CLI into a global-grade database transformation library and operational migration platform.

The current repository is public. Therefore, this directory is only "hidden" in the sense that it is not linked from public-facing documentation. It must not contain secrets, credentials, customer names, private infrastructure paths, production hostnames, or unpublished commercial commitments.

## Documents

- `001_GLOBAL_LIBRARY_READINESS_AUDIT.md` — gap analysis between current capability and global library expectations.
- `002_ENTERPRISE_MIGRATION_ARCHITECTURE.md` — target technical architecture for resumable, auditable, high-volume, bidirectional database transformation.
- `003_DISCOVERABILITY_AND_AI_INDEXING_STRATEGY.md` — staged strategy for search, GitHub, Go package, and AI discoverability after the library is ready.

## Internal rules

1. Treat this directory as an execution contract, not as marketing copy.
2. Every claim about QantaraDB capabilities must be split into:
   - implemented now,
   - partially implemented,
   - planned,
   - explicitly not supported yet.
3. Do not expose unsupported claims in public README, website, package docs, or release text.
4. Do not add credential examples containing real users, passwords, hosts, ports, customer database names, or environment-specific paths.
5. Before public launch, convert the stable parts of these documents into public docs under `docs/` and keep experimental sections internal.
