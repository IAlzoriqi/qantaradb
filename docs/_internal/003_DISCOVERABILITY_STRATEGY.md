# QantaraDB Discoverability Strategy

Status: internal early-development draft.

## Purpose

Prepare public documentation so developers can understand QantaraDB accurately when the project is ready for wider release.

## Required corrections before public promotion

- Correct the Go module path to `github.com/IAlzoriqi/qantaradb`.
- Add package comments for Go documentation.
- Add runnable examples.
- Add release tags.
- Add integration tests.
- Add contribution and security documents.
- Keep implemented features separate from planned features.

## Public wording rule

Public docs may describe only implemented and tested functionality. Planned features must be clearly marked as planned.

## Recommended public description

```text
Safe, resumable Go library and CLI for MySQL/MariaDB to PostgreSQL database migration, schema conversion, validation reports, and planned bidirectional support.
```

## Suggested repository topics

```text
mysql
mariadb
postgresql
database-migration
schema-migration
data-migration
migration-tool
go
cli
resumable
checksum
validation
laravel
```

## Future public docs

```text
docs/index.md
docs/getting-started.md
docs/installation.md
docs/configuration.md
docs/mysql-to-postgres.md
docs/safety-model.md
docs/resume-and-recovery.md
docs/validation-reports.md
docs/type-mapping.md
docs/examples.md
docs/cli-reference.md
docs/library-api.md
docs/roadmap.md
```

## Future website files

When a public documentation site is created, prepare:

- `sitemap.xml`
- `robots.txt`
- `llms.txt`
- `llms-full.txt`

These files must not include internal docs, private paths, secrets, customer names, or unsupported claims.

## Launch checklist

```text
[ ] module path corrected
[ ] README updated
[ ] package comments added
[ ] examples added
[ ] integration tests added
[ ] release tag added
[ ] pkg.go.dev page available
[ ] public docs site prepared
[ ] repository topics configured
[ ] SECURITY.md added
[ ] CONTRIBUTING.md added
[ ] CHANGELOG.md added
```
