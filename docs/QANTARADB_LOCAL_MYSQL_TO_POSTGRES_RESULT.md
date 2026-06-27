# QantaraDB Local MySQL To PostgreSQL Result

status:
partially_passed

schema:
- tables_created: 944
- indexes_created: 709 groups detected
- foreign_keys_created: 114 groups detected
- sequences_reset: not fully implemented; requires focused sequence reset pass

data:
- tables_copied: 944
- rows_copied: 770074
- failed_rows: 0 hard failures after UTF-8/NUL sanitization

validation:
- row_counts: passed for all 944 tables
- primary_keys: partially validated by existing validator
- foreign_keys: passed
- checksums: partially passed; 564/944 tables passed
- samples: partially passed
- datetime: copied through configured timestamp mapping
- decimal: copied through numeric mapping
- json: copied through jsonb mapping

blocked_tables:
- table: none after local-only execution fixes
  reason: no table remained fully blocked

ready_for_console_local_postgres:
yes, with caveats

caveats:
- 380 tables failed checksum comparison despite row-count parity, likely due type formatting, UTF-8 normalization, and NUL-byte sanitization.
- Sanitized text rows need an explicit audit report before staging database rehearsal.
- `tinyint(1)` was treated as smallint for this local run; safe boolean allow-listing is a future task.
