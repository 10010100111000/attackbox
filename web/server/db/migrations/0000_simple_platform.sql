CREATE TABLE "auth_sessions" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"user_id" uuid NOT NULL,
	"token_hash" "bytea" NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"expires_at" timestamp with time zone NOT NULL,
	"last_seen_at" timestamp with time zone DEFAULT now() NOT NULL,
	"revoked_at" timestamp with time zone,
	"ip" "inet",
	"user_agent" varchar(512),
	CONSTRAINT "auth_sessions_token_hash_len_check" CHECK (octet_length("auth_sessions"."token_hash") = 32),
	CONSTRAINT "auth_sessions_expires_after_created_check" CHECK ("auth_sessions"."expires_at" > "auth_sessions"."created_at")
);
--> statement-breakpoint
CREATE TABLE "email_tokens" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"user_id" uuid NOT NULL,
	"target_email" varchar(254) NOT NULL,
	"purpose" varchar(32) NOT NULL,
	"token_hash" "bytea" NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"expires_at" timestamp with time zone NOT NULL,
	"consumed_at" timestamp with time zone,
	CONSTRAINT "email_tokens_purpose_check" CHECK ("email_tokens"."purpose" in ('verify_email', 'reset_password')),
	CONSTRAINT "email_tokens_token_hash_len_check" CHECK (octet_length("email_tokens"."token_hash") = 32),
	CONSTRAINT "email_tokens_expires_after_created_check" CHECK ("email_tokens"."expires_at" > "email_tokens"."created_at")
);
--> statement-breakpoint
CREATE TABLE "users" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"email" varchar(254) NOT NULL,
	"display_name" varchar(64) NOT NULL,
	"password_hash" text,
	"role" varchar(16) DEFAULT 'user' NOT NULL,
	"status" varchar(16) DEFAULT 'pending' NOT NULL,
	"email_verified_at" timestamp with time zone,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"last_login_at" timestamp with time zone,
	CONSTRAINT "users_email_lower_trimmed_check" CHECK ("users"."email" = lower(btrim("users"."email"))),
	CONSTRAINT "users_display_name_non_empty_check" CHECK (length(btrim("users"."display_name")) > 0),
	CONSTRAINT "users_role_check" CHECK ("users"."role" in ('user', 'admin')),
	CONSTRAINT "users_status_check" CHECK ("users"."status" in ('pending', 'active', 'suspended'))
);
--> statement-breakpoint
CREATE TABLE "audit_logs" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"actor_user_id" uuid,
	"action" varchar(80) NOT NULL,
	"target_type" varchar(40) NOT NULL,
	"target_id" uuid,
	"metadata" jsonb DEFAULT '{}'::jsonb NOT NULL,
	"request_id" uuid,
	"ip" "inet",
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
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
CREATE TABLE "chapter_assets" (
	"chapter_id" uuid NOT NULL,
	"asset_id" uuid NOT NULL,
	CONSTRAINT "chapter_assets_chapter_id_asset_id_pk" PRIMARY KEY("chapter_id","asset_id")
);
--> statement-breakpoint
CREATE TABLE "chapter_questions" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"chapter_id" uuid NOT NULL,
	"position" integer NOT NULL,
	"prompt_markdown" text NOT NULL,
	"error_hint" text,
	"normalization" varchar(20) DEFAULT 'trim' NOT NULL,
	"answer_hmac" "bytea",
	"answer_key_version" smallint DEFAULT 1 NOT NULL,
	CONSTRAINT "chapter_questions_normalization_check" CHECK ("chapter_questions"."normalization" in ('exact', 'trim', 'trim_casefold')),
	CONSTRAINT "chapter_questions_hmac_check" CHECK ("chapter_questions"."answer_hmac" is null or octet_length("chapter_questions"."answer_hmac") = 32),
	CONSTRAINT "chapter_questions_position_check" CHECK ("chapter_questions"."position" > 0),
	CONSTRAINT "chapter_questions_key_version_check" CHECK ("chapter_questions"."answer_key_version" > 0)
);
--> statement-breakpoint
CREATE TABLE "chapters" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"module_id" uuid NOT NULL,
	"slug" varchar(100) NOT NULL,
	"position" integer NOT NULL,
	"title" varchar(160) NOT NULL,
	"duration_minutes" integer,
	"access" varchar(20) DEFAULT 'public' NOT NULL,
	"body_markdown" text DEFAULT '' NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "chapters_slug_check" CHECK ("chapters"."slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
	CONSTRAINT "chapters_position_check" CHECK ("chapters"."position" > 0),
	CONSTRAINT "chapters_duration_check" CHECK ("chapters"."duration_minutes" is null or "chapters"."duration_minutes" between 1 and 1440),
	CONSTRAINT "chapters_access_check" CHECK ("chapters"."access" in ('public', 'authenticated'))
);
--> statement-breakpoint
CREATE TABLE "module_prerequisites" (
	"module_id" uuid NOT NULL,
	"prerequisite_id" uuid NOT NULL,
	"position" integer NOT NULL,
	CONSTRAINT "module_prerequisites_module_id_prerequisite_id_pk" PRIMARY KEY("module_id","prerequisite_id"),
	CONSTRAINT "module_prerequisites_self_check" CHECK ("module_prerequisites"."module_id" <> "module_prerequisites"."prerequisite_id"),
	CONSTRAINT "module_prerequisites_position_check" CHECK ("module_prerequisites"."position" > 0)
);
--> statement-breakpoint
CREATE TABLE "modules" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"slug" varchar(100) NOT NULL,
	"title" varchar(160) NOT NULL,
	"summary" varchar(500) DEFAULT '' NOT NULL,
	"description_markdown" text DEFAULT '' NOT NULL,
	"learning_outcomes" text[] DEFAULT '{}'::text[] NOT NULL,
	"difficulty" varchar(20) DEFAULT 'beginner' NOT NULL,
	"category_id" uuid,
	"cover_asset_id" uuid,
	"status" varchar(20) DEFAULT 'unpublished' NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"created_by" uuid NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"published_at" timestamp with time zone,
	CONSTRAINT "modules_slug_check" CHECK ("modules"."slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
	CONSTRAINT "modules_status_check" CHECK ("modules"."status" in ('unpublished', 'published')),
	CONSTRAINT "modules_difficulty_check" CHECK ("modules"."difficulty" in ('beginner', 'intermediate', 'advanced')),
	CONSTRAINT "modules_lock_version_check" CHECK ("modules"."lock_version" > 0)
);
--> statement-breakpoint
CREATE TABLE "content_categories" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"slug" varchar(100) NOT NULL,
	"name" varchar(80) NOT NULL,
	"position" integer DEFAULT 0 NOT NULL,
	CONSTRAINT "content_categories_slug_check" CHECK ("content_categories"."slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);
--> statement-breakpoint
ALTER TABLE "auth_sessions" ADD CONSTRAINT "auth_sessions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "email_tokens" ADD CONSTRAINT "email_tokens_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "audit_logs" ADD CONSTRAINT "audit_logs_actor_user_id_users_id_fk" FOREIGN KEY ("actor_user_id") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "assets" ADD CONSTRAINT "assets_uploaded_by_users_id_fk" FOREIGN KEY ("uploaded_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_assets" ADD CONSTRAINT "chapter_assets_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_assets" ADD CONSTRAINT "chapter_assets_asset_id_assets_id_fk" FOREIGN KEY ("asset_id") REFERENCES "public"."assets"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_questions" ADD CONSTRAINT "chapter_questions_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapters" ADD CONSTRAINT "chapters_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_prerequisites" ADD CONSTRAINT "module_prerequisites_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_prerequisites" ADD CONSTRAINT "module_prerequisites_prerequisite_id_modules_id_fk" FOREIGN KEY ("prerequisite_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_category_id_content_categories_id_fk" FOREIGN KEY ("category_id") REFERENCES "public"."content_categories"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_cover_asset_id_assets_id_fk" FOREIGN KEY ("cover_asset_id") REFERENCES "public"."assets"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "auth_sessions_token_hash_unique_idx" ON "auth_sessions" USING btree ("token_hash");--> statement-breakpoint
CREATE INDEX "auth_sessions_user_expires_idx" ON "auth_sessions" USING btree ("user_id","expires_at");--> statement-breakpoint
CREATE UNIQUE INDEX "email_tokens_token_hash_unique_idx" ON "email_tokens" USING btree ("token_hash");--> statement-breakpoint
CREATE UNIQUE INDEX "users_email_unique_idx" ON "users" USING btree ("email");--> statement-breakpoint
CREATE INDEX "audit_logs_target_created_idx" ON "audit_logs" USING btree ("target_type","target_id","created_at");--> statement-breakpoint
CREATE INDEX "audit_logs_actor_created_idx" ON "audit_logs" USING btree ("actor_user_id","created_at");--> statement-breakpoint
CREATE UNIQUE INDEX "assets_storage_key_unique_idx" ON "assets" USING btree ("storage_key");--> statement-breakpoint
CREATE INDEX "assets_sha256_idx" ON "assets" USING btree ("sha256");--> statement-breakpoint
CREATE INDEX "assets_uploaded_by_idx" ON "assets" USING btree ("uploaded_by");--> statement-breakpoint
CREATE INDEX "chapter_assets_asset_idx" ON "chapter_assets" USING btree ("asset_id");--> statement-breakpoint
CREATE UNIQUE INDEX "chapter_questions_position_unique" ON "chapter_questions" USING btree ("chapter_id","position");--> statement-breakpoint
CREATE UNIQUE INDEX "chapters_module_slug_unique" ON "chapters" USING btree ("module_id","slug");--> statement-breakpoint
CREATE UNIQUE INDEX "chapters_module_position_unique" ON "chapters" USING btree ("module_id","position");--> statement-breakpoint
CREATE UNIQUE INDEX "module_prerequisites_position_unique" ON "module_prerequisites" USING btree ("module_id","position");--> statement-breakpoint
CREATE UNIQUE INDEX "modules_slug_unique_idx" ON "modules" USING btree ("slug");--> statement-breakpoint
CREATE INDEX "modules_status_idx" ON "modules" USING btree ("status");--> statement-breakpoint
CREATE UNIQUE INDEX "content_categories_slug_unique" ON "content_categories" USING btree ("slug");--> statement-breakpoint
CREATE UNIQUE INDEX "content_categories_name_unique" ON "content_categories" USING btree ("name");

--> statement-breakpoint
INSERT INTO content_categories (slug, name, position) VALUES
('web-security', 'Web 安全', 1), ('linux', 'Linux 基础', 2),
('network', '网络安全', 3), ('cloud', '云原生安全', 4),
('reverse', '逆向分析', 5), ('containers', '容器安全', 6)
ON CONFLICT (slug) DO NOTHING;
