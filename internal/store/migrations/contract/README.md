# Contract migrations (SPEC-CORE-73.R16)

SQL files here remove or change structure (`DROP`, `RENAME`, `ALTER COLUMN ... TYPE`,
`SET NOT NULL`) or build indexes on large tables with `CREATE INDEX CONCURRENTLY`.

They never run when the core starts. Apply them with `aipods-cli migrate contract`
once the previous color no longer exists, so the code still serving never sees
a column disappear. Each file is recorded in `schema_migrations` as `contract/<name>`.

A file that contains `CONCURRENTLY` runs outside a transaction and must hold a
single statement.
