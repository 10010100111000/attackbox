CREATE TABLE "assets" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"storage_key" text NOT NULL,
	"original_name" varchar(255) NOT NULL,
	"media_type" varchar(120) NOT NULL,
	"byte_size" bigint NOT NULL,
	"sha256" "bytea" NOT NULL,
	"visibility" varchar(16) NOT NULL,
	"uploaded_by" uuid NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"archived_at" timestamp with time zone,
	CONSTRAINT "assets_visibility_check" CHECK ("assets"."visibility" in ('public', 'private')),
	CONSTRAINT "assets_byte_size_check" CHECK ("assets"."byte_size" >= 0),
	CONSTRAINT "assets_sha256_len_check" CHECK (octet_length("assets"."sha256") = 32)
);
--> statement-breakpoint
CREATE TABLE "chapter_questions" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"chapter_revision_id" uuid NOT NULL,
	"question_key" uuid DEFAULT gen_random_uuid() NOT NULL,
	"position" integer NOT NULL,
	"prompt_markdown" text NOT NULL,
	"error_hint" text,
	"normalization" varchar(20) DEFAULT 'trim' NOT NULL,
	"answer_hmac" "bytea",
	"answer_key_version" smallint DEFAULT 1 NOT NULL,
	CONSTRAINT "chapter_questions_normalization_check" CHECK ("chapter_questions"."normalization" in ('exact', 'trim', 'trim_casefold')),
	CONSTRAINT "chapter_questions_answer_hmac_len_check" CHECK ("chapter_questions"."answer_hmac" is null or octet_length("chapter_questions"."answer_hmac") = 32),
	CONSTRAINT "chapter_questions_position_check" CHECK ("chapter_questions"."position" > 0),
	CONSTRAINT "chapter_questions_key_version_check" CHECK ("chapter_questions"."answer_key_version" > 0)
);
--> statement-breakpoint
CREATE TABLE "chapter_revision_assets" (
	"chapter_revision_id" uuid NOT NULL,
	"asset_id" uuid NOT NULL,
	CONSTRAINT "chapter_revision_assets_pk" PRIMARY KEY("chapter_revision_id","asset_id")
);
--> statement-breakpoint
CREATE TABLE "chapter_revisions" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"chapter_id" uuid NOT NULL,
	"title" varchar(160) NOT NULL,
	"access" varchar(20) DEFAULT 'public' NOT NULL,
	"body_markdown" text DEFAULT '' NOT NULL,
	"source_kind" varchar(20) DEFAULT 'editor' NOT NULL,
	"source_url" text,
	"source_commit" varchar(64),
	"source_sha256" "bytea",
	"is_sealed" boolean DEFAULT false NOT NULL,
	"created_by" uuid NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "chapter_revisions_access_check" CHECK ("chapter_revisions"."access" in ('public', 'authenticated')),
	CONSTRAINT "chapter_revisions_source_kind_check" CHECK ("chapter_revisions"."source_kind" in ('editor', 'upload', 'github')),
	CONSTRAINT "chapter_revisions_sha256_len_check" CHECK ("chapter_revisions"."source_sha256" is null or octet_length("chapter_revisions"."source_sha256") = 32),
	CONSTRAINT "chapter_revisions_id_chapter_id_unique" UNIQUE("id","chapter_id")
);
--> statement-breakpoint
CREATE TABLE "chapters" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"module_id" uuid NOT NULL,
	"slug" varchar(100) NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"archived_at" timestamp with time zone,
	CONSTRAINT "chapters_slug_format_check" CHECK ("chapters"."slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
	CONSTRAINT "chapters_module_slug_unique" UNIQUE("module_id","slug"),
	CONSTRAINT "chapters_id_module_id_unique" UNIQUE("id","module_id")
);
--> statement-breakpoint
CREATE TABLE "module_revision_chapters" (
	"module_revision_id" uuid NOT NULL,
	"module_id" uuid NOT NULL,
	"chapter_id" uuid NOT NULL,
	"chapter_revision_id" uuid NOT NULL,
	"position" integer NOT NULL,
	CONSTRAINT "module_revision_chapters_pk" PRIMARY KEY("module_revision_id","chapter_id"),
	CONSTRAINT "module_revision_chapters_pos_check" CHECK ("module_revision_chapters"."position" > 0)
);
--> statement-breakpoint
CREATE TABLE "module_revisions" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"module_id" uuid NOT NULL,
	"revision_no" integer NOT NULL,
	"state" varchar(16) NOT NULL,
	"title" varchar(160),
	"summary" varchar(500),
	"description_markdown" text,
	"cover_asset_id" uuid,
	"difficulty" varchar(20) DEFAULT 'beginner' NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"created_by" uuid NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"published_at" timestamp with time zone,
	CONSTRAINT "module_revisions_state_check" CHECK ("module_revisions"."state" in ('draft', 'published')),
	CONSTRAINT "module_revisions_difficulty_check" CHECK ("module_revisions"."difficulty" in ('beginner', 'intermediate', 'advanced')),
	CONSTRAINT "module_revisions_revision_no_check" CHECK ("module_revisions"."revision_no" > 0),
	CONSTRAINT "module_revisions_lock_version_check" CHECK ("module_revisions"."lock_version" >= 1),
	CONSTRAINT "module_revisions_module_rev_no_unique" UNIQUE("module_id","revision_no"),
	CONSTRAINT "module_revisions_id_module_id_unique" UNIQUE("id","module_id")
);
--> statement-breakpoint
CREATE TABLE "modules" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"slug" varchar(100) NOT NULL,
	"published_revision_id" uuid,
	"created_by" uuid NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"archived_at" timestamp with time zone,
	CONSTRAINT "modules_slug_format_check" CHECK ("modules"."slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);
--> statement-breakpoint
ALTER TABLE "assets" ADD CONSTRAINT "assets_uploaded_by_users_id_fk" FOREIGN KEY ("uploaded_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_questions" ADD CONSTRAINT "chapter_questions_chapter_revision_id_chapter_revisions_id_fk" FOREIGN KEY ("chapter_revision_id") REFERENCES "public"."chapter_revisions"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_revision_assets" ADD CONSTRAINT "chapter_revision_assets_chapter_revision_id_chapter_revisions_id_fk" FOREIGN KEY ("chapter_revision_id") REFERENCES "public"."chapter_revisions"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_revision_assets" ADD CONSTRAINT "chapter_revision_assets_asset_id_assets_id_fk" FOREIGN KEY ("asset_id") REFERENCES "public"."assets"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_revisions" ADD CONSTRAINT "chapter_revisions_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_revisions" ADD CONSTRAINT "chapter_revisions_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapters" ADD CONSTRAINT "chapters_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revision_chapters" ADD CONSTRAINT "module_revision_chapters_module_revision_id_module_revisions_id_fk" FOREIGN KEY ("module_revision_id") REFERENCES "public"."module_revisions"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revision_chapters" ADD CONSTRAINT "module_revision_chapters_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revision_chapters" ADD CONSTRAINT "module_revision_chapters_chapter_revision_id_chapter_revisions_id_fk" FOREIGN KEY ("chapter_revision_id") REFERENCES "public"."chapter_revisions"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revision_chapters" ADD CONSTRAINT "mrc_module_revision_fk" FOREIGN KEY ("module_revision_id","module_id") REFERENCES "public"."module_revisions"("id","module_id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revision_chapters" ADD CONSTRAINT "mrc_chapter_fk" FOREIGN KEY ("chapter_id","module_id") REFERENCES "public"."chapters"("id","module_id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revision_chapters" ADD CONSTRAINT "mrc_chapter_revision_fk" FOREIGN KEY ("chapter_revision_id","chapter_id") REFERENCES "public"."chapter_revisions"("id","chapter_id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revisions" ADD CONSTRAINT "module_revisions_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revisions" ADD CONSTRAINT "module_revisions_cover_asset_id_assets_id_fk" FOREIGN KEY ("cover_asset_id") REFERENCES "public"."assets"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_revisions" ADD CONSTRAINT "module_revisions_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_published_rev_fk" FOREIGN KEY ("published_revision_id","id") REFERENCES "public"."module_revisions"("id","module_id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "assets_storage_key_unique_idx" ON "assets" USING btree ("storage_key");--> statement-breakpoint
CREATE INDEX "assets_sha256_idx" ON "assets" USING btree ("sha256");--> statement-breakpoint
CREATE INDEX "assets_uploaded_by_idx" ON "assets" USING btree ("uploaded_by");--> statement-breakpoint
CREATE UNIQUE INDEX "chapter_questions_rev_pos_unique_idx" ON "chapter_questions" USING btree ("chapter_revision_id","position");--> statement-breakpoint
CREATE UNIQUE INDEX "chapter_questions_rev_key_unique_idx" ON "chapter_questions" USING btree ("chapter_revision_id","question_key");--> statement-breakpoint
CREATE INDEX "chapter_revision_assets_asset_idx" ON "chapter_revision_assets" USING btree ("asset_id");--> statement-breakpoint
CREATE INDEX "chapter_revisions_chapter_sealed_idx" ON "chapter_revisions" USING btree ("chapter_id","is_sealed");--> statement-breakpoint
CREATE UNIQUE INDEX "module_revision_chapters_pos_unique_idx" ON "module_revision_chapters" USING btree ("module_revision_id","position");--> statement-breakpoint
CREATE UNIQUE INDEX "module_revisions_single_draft_idx" ON "module_revisions" USING btree ("module_id") WHERE "module_revisions"."state" = 'draft';--> statement-breakpoint
CREATE UNIQUE INDEX "modules_slug_unique_idx" ON "modules" USING btree ("slug");--> statement-breakpoint
CREATE INDEX "modules_published_revision_idx" ON "modules" USING btree ("published_revision_id");
