-- Remove only untouched, unreferenced seed data. Real module classifications survive.
DELETE FROM content_categories AS category
USING (VALUES
  ('web-security', 'Web 安全', 1), ('linux', 'Linux 基础', 2),
  ('network', '网络安全', 3), ('cloud', '云原生安全', 4),
  ('reverse', '逆向分析', 5), ('containers', '容器安全', 6)
) AS seed(slug, name, position)
WHERE category.slug = seed.slug AND category.name = seed.name
  AND category.position = seed.position
  AND NOT EXISTS (SELECT 1 FROM modules WHERE category_id = category.id);
