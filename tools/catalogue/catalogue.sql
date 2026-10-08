-- The catalogue query of specarch extract database, for PostgreSQL 12 and
-- later. It prints one JSON document: every table, view and enum type of
-- every schema but the system ones, each list in the order of its names,
-- so that the same database gives the same bytes. Run it with psql, giving
-- the path the migrations were read from and the commit that last changed
-- it:
--
--   psql -X -q -A -t -v path=<path> -v commit=<hash> -f catalogue.sql
--
-- dump-catalogue.sh beside it does that against a disposable database.

WITH
schemas AS (
  SELECT n.oid, n.nspname
  FROM pg_namespace n
  WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
    AND n.nspname NOT LIKE 'pg_temp_%'
    AND n.nspname NOT LIKE 'pg_toast_temp_%'
),
tables AS (
  SELECT c.oid, s.nspname AS schema, c.relname AS name, obj_description(c.oid, 'pg_class') AS comment
  FROM pg_class c JOIN schemas s ON s.oid = c.relnamespace
  WHERE c.relkind IN ('r', 'p')
),
columns AS (
  SELECT a.attrelid AS table_oid, a.attnum,
    json_build_object(
      'name', a.attname,
      'type', format_type(a.atttypid, a.atttypmod),
      'notNull', a.attnotnull,
      'default', pg_get_expr(d.adbin, d.adrelid),
      'identity', NULLIF(a.attidentity, ''),
      'generated', NULLIF(a.attgenerated, ''),
      'comment', col_description(a.attrelid, a.attnum)
    ) AS doc
  FROM pg_attribute a
  JOIN tables t ON t.oid = a.attrelid
  LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
  WHERE a.attnum > 0 AND NOT a.attisdropped
),
constraints AS (
  SELECT k.conrelid AS table_oid, k.conname,
    json_build_object(
      'name', k.conname,
      'kind', CASE k.contype WHEN 'p' THEN 'primary' WHEN 'f' THEN 'foreign' WHEN 'u' THEN 'unique' WHEN 'c' THEN 'check' WHEN 'x' THEN 'exclusion' ELSE k.contype::text END,
      'columns', (SELECT json_agg(a.attname ORDER BY u.ord) FROM unnest(k.conkey) WITH ORDINALITY u(num, ord) JOIN pg_attribute a ON a.attrelid = k.conrelid AND a.attnum = u.num),
      'definition', pg_get_constraintdef(k.oid),
      'referencesSchema', rn.nspname,
      'referencesTable', rc.relname,
      'referencesColumns', (SELECT json_agg(a.attname ORDER BY u.ord) FROM unnest(k.confkey) WITH ORDINALITY u(num, ord) JOIN pg_attribute a ON a.attrelid = k.confrelid AND a.attnum = u.num),
      'onDelete', CASE WHEN k.contype = 'f' THEN CASE k.confdeltype WHEN 'a' THEN 'no action' WHEN 'r' THEN 'restrict' WHEN 'c' THEN 'cascade' WHEN 'n' THEN 'set null' WHEN 'd' THEN 'set default' END END
    ) AS doc
  FROM pg_constraint k
  JOIN tables t ON t.oid = k.conrelid
  LEFT JOIN pg_class rc ON rc.oid = k.confrelid
  LEFT JOIN pg_namespace rn ON rn.oid = rc.relnamespace
  WHERE k.contype IN ('p', 'f', 'u', 'c', 'x')
),
indexes AS (
  SELECT i.indrelid AS table_oid, ic.relname,
    json_build_object(
      'name', ic.relname,
      'unique', i.indisunique,
      'primary', i.indisprimary,
      'backsConstraint', EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conindid = i.indexrelid AND k.conrelid = i.indrelid),
      'definition', pg_get_indexdef(i.indexrelid)
    ) AS doc
  FROM pg_index i
  JOIN tables t ON t.oid = i.indrelid
  JOIN pg_class ic ON ic.oid = i.indexrelid
)
SELECT jsonb_pretty(jsonb_build_object(
  'catalogue', 'postgresql',
  'path', :'path',
  'commit', :'commit',
  'tables', COALESCE((
    SELECT jsonb_agg(jsonb_build_object(
      'schema', t.schema,
      'name', t.name,
      'comment', t.comment,
      'columns', COALESCE((SELECT json_agg(c.doc ORDER BY c.attnum) FROM columns c WHERE c.table_oid = t.oid), '[]'::json),
      'constraints', COALESCE((SELECT json_agg(k.doc ORDER BY k.conname) FROM constraints k WHERE k.table_oid = t.oid), '[]'::json),
      'indexes', COALESCE((SELECT json_agg(i.doc ORDER BY i.relname) FROM indexes i WHERE i.table_oid = t.oid), '[]'::json)
    ) ORDER BY t.schema, t.name)
    FROM tables t), '[]'::jsonb),
  'views', COALESCE((
    SELECT jsonb_agg(jsonb_build_object('schema', s.nspname, 'name', c.relname, 'materialized', c.relkind = 'm') ORDER BY s.nspname, c.relname)
    FROM pg_class c JOIN schemas s ON s.oid = c.relnamespace
    WHERE c.relkind IN ('v', 'm')), '[]'::jsonb),
  'enums', COALESCE((
    SELECT jsonb_agg(jsonb_build_object(
      'schema', s.nspname,
      'name', ty.typname,
      'values', (SELECT json_agg(e.enumlabel ORDER BY e.enumsortorder) FROM pg_enum e WHERE e.enumtypid = ty.oid)
    ) ORDER BY s.nspname, ty.typname)
    FROM pg_type ty JOIN schemas s ON s.oid = ty.typnamespace
    WHERE ty.typtype = 'e'), '[]'::jsonb)
));
