CREATE TABLE "content_categories" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"slug" varchar(100) NOT NULL,
	"name" varchar(80) NOT NULL,
	"position" integer DEFAULT 0 NOT NULL,
	CONSTRAINT "content_categories_slug_check" CHECK ("content_categories"."slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);
--> statement-breakpoint
ALTER TABLE "module_revisions" ADD COLUMN "category_id" uuid;--> statement-breakpoint
CREATE UNIQUE INDEX "content_categories_slug_unique" ON "content_categories" USING btree ("slug");--> statement-breakpoint
CREATE UNIQUE INDEX "content_categories_name_unique" ON "content_categories" USING btree ("name");--> statement-breakpoint
ALTER TABLE "module_revisions" ADD CONSTRAINT "module_revisions_category_id_content_categories_id_fk" FOREIGN KEY ("category_id") REFERENCES "public"."content_categories"("id") ON DELETE restrict ON UPDATE no action;
--> statement-breakpoint
INSERT INTO content_categories (slug, name, position) VALUES
('web-security', 'Web 安全', 1), ('linux', 'Linux 基础', 2),
('network', '网络安全', 3), ('cloud', '云原生安全', 4),
('reverse', '逆向分析', 5), ('containers', '容器安全', 6)
ON CONFLICT (slug) DO NOTHING;
