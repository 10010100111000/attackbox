CREATE TABLE "chapter_sources" (
	"chapter_id" uuid PRIMARY KEY NOT NULL,
	"module_id" uuid NOT NULL,
	"match_key" text NOT NULL
);
--> statement-breakpoint
CREATE TABLE "content_previews" (
	"id" uuid PRIMARY KEY NOT NULL,
	"actor_id" uuid NOT NULL,
	"origin" text NOT NULL,
	"commit" text,
	"config_version" integer,
	"baseline" text NOT NULL,
	"manifest" text NOT NULL,
	"bindings" jsonb NOT NULL,
	"result" jsonb,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"expires_at" timestamp with time zone NOT NULL
);
--> statement-breakpoint
CREATE TABLE "content_repository" (
	"id" integer PRIMARY KEY NOT NULL,
	"repository" text NOT NULL,
	"branch" text NOT NULL,
	"directory" text NOT NULL,
	"token_cipher" text NOT NULL,
	"version" integer DEFAULT 1 NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "module_sources" (
	"module_id" uuid PRIMARY KEY NOT NULL,
	"origin" text NOT NULL,
	"source_path" text NOT NULL,
	"source_hash" text NOT NULL,
	"commit" text,
	"synced_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "question_sources" (
	"question_id" uuid PRIMARY KEY NOT NULL,
	"chapter_id" uuid NOT NULL,
	"match_key" text NOT NULL
);
--> statement-breakpoint
ALTER TABLE "chapter_sources" ADD CONSTRAINT "chapter_sources_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_sources" ADD CONSTRAINT "chapter_sources_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "content_previews" ADD CONSTRAINT "content_previews_actor_id_users_id_fk" FOREIGN KEY ("actor_id") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_sources" ADD CONSTRAINT "module_sources_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "question_sources" ADD CONSTRAINT "question_sources_question_id_chapter_questions_id_fk" FOREIGN KEY ("question_id") REFERENCES "public"."chapter_questions"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "question_sources" ADD CONSTRAINT "question_sources_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "chapter_sources_module_key_unique" ON "chapter_sources" USING btree ("module_id","match_key");--> statement-breakpoint
CREATE UNIQUE INDEX "module_sources_origin_path_unique" ON "module_sources" USING btree ("origin","source_path");--> statement-breakpoint
CREATE UNIQUE INDEX "question_sources_chapter_key_unique" ON "question_sources" USING btree ("chapter_id","match_key");