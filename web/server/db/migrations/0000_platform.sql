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
	"answer" text,
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
	"target_mode" varchar(16) DEFAULT 'inherit' NOT NULL,
	"target_template_id" uuid,
	"access" varchar(20) DEFAULT 'public' NOT NULL,
	"body_markdown" text DEFAULT '' NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "chapters_slug_check" CHECK ("chapters"."slug" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
	CONSTRAINT "chapters_position_check" CHECK ("chapters"."position" > 0),
	CONSTRAINT "chapters_target_check" CHECK (("chapters"."target_mode"='custom' and "chapters"."target_template_id" is not null) or ("chapters"."target_mode" in ('inherit','none') and "chapters"."target_template_id" is null)),
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
	"default_target_template_id" uuid,
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
CREATE TABLE "chapter_progress" (
	"user_id" uuid NOT NULL,
	"chapter_id" uuid NOT NULL,
	"started_at" timestamp with time zone DEFAULT now() NOT NULL,
	"last_visited_at" timestamp with time zone DEFAULT now() NOT NULL,
	"completed_at" timestamp with time zone,
	CONSTRAINT "chapter_progress_user_id_chapter_id_pk" PRIMARY KEY("user_id","chapter_id")
);
--> statement-breakpoint
CREATE TABLE "question_completions" (
	"user_id" uuid NOT NULL,
	"question_id" uuid NOT NULL,
	"passed_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "question_completions_user_id_question_id_pk" PRIMARY KEY("user_id","question_id")
);
--> statement-breakpoint
CREATE TABLE "sandbox_identities" (
	"user_id" uuid PRIMARY KEY NOT NULL,
	"worker_user_id" bigserial NOT NULL,
	CONSTRAINT "sandbox_identity_safe_integer_check" CHECK ("sandbox_identities"."worker_user_id" > 0 and "sandbox_identities"."worker_user_id" <= 9007199254740991)
);
--> statement-breakpoint
CREATE TABLE "sandbox_node_settings" (
	"node_id" varchar(64) PRIMARY KEY NOT NULL,
	"label" varchar(80) NOT NULL,
	"max_slots" integer NOT NULL,
	"memory_mb" integer NOT NULL,
	"pids_max" integer NOT NULL,
	"draining" boolean DEFAULT false NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_by" uuid,
	CONSTRAINT "sandbox_node_limits_check" CHECK ("sandbox_node_settings"."max_slots" between 1 and 250 and "sandbox_node_settings"."memory_mb" between 128 and 65536 and "sandbox_node_settings"."pids_max" between 32 and 4096),
	CONSTRAINT "sandbox_node_version_check" CHECK ("sandbox_node_settings"."lock_version" > 0),
	CONSTRAINT "sandbox_node_label_check" CHECK (length(btrim("sandbox_node_settings"."label")) > 0)
);
--> statement-breakpoint
CREATE TABLE "learning_route_modules" (
	"route_id" uuid NOT NULL,
	"module_id" uuid NOT NULL,
	"position" integer NOT NULL,
	CONSTRAINT "learning_route_modules_route_id_module_id_pk" PRIMARY KEY("route_id","module_id"),
	CONSTRAINT "learning_route_modules_route_id_position_unique" UNIQUE("route_id","position"),
	CONSTRAINT "learning_route_modules_position_check" CHECK ("learning_route_modules"."position">0)
);
--> statement-breakpoint
CREATE TABLE "learning_routes" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"title" varchar(160) NOT NULL,
	"summary" varchar(500) DEFAULT '' NOT NULL,
	"status" varchar(20) DEFAULT 'unpublished' NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "learning_routes_title_check" CHECK (length(trim("learning_routes"."title"))>0),
	CONSTRAINT "learning_routes_status_check" CHECK ("learning_routes"."status" in ('unpublished','published')),
	CONSTRAINT "learning_routes_lock_version_check" CHECK ("learning_routes"."lock_version">0)
);
--> statement-breakpoint
CREATE TABLE "target_templates" (
	"revision" integer DEFAULT 1 NOT NULL,
	"bundle_id" uuid NOT NULL,
	"settings" jsonb NOT NULL,
	"variables_cipher" text,
	"variable_keys" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"name" varchar(120) NOT NULL,
	"description" varchar(500) DEFAULT '' NOT NULL,
	"access_instructions" text DEFAULT '' NOT NULL,
	"enabled" boolean DEFAULT false NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "target_templates_name_check" CHECK (length(trim("target_templates"."name"))>0),
	CONSTRAINT "target_templates_lock_check" CHECK ("target_templates"."lock_version">0),
	CONSTRAINT "target_templates_revision_check" CHECK ("target_templates"."revision">0),
	CONSTRAINT "target_templates_settings_check" CHECK (jsonb_typeof("target_templates"."settings")='object')
);
--> statement-breakpoint
CREATE TABLE "challenge_attempt_limits" (
	"user_id" uuid NOT NULL,
	"challenge_id" uuid NOT NULL,
	"window_at" timestamp with time zone DEFAULT now() NOT NULL,
	"attempts" integer DEFAULT 0 NOT NULL,
	CONSTRAINT "challenge_attempt_limits_user_id_challenge_id_pk" PRIMARY KEY("user_id","challenge_id")
);
--> statement-breakpoint
CREATE TABLE "challenge_completions" (
	"user_id" uuid NOT NULL,
	"stage_id" uuid NOT NULL,
	"passed_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "challenge_completions_user_id_stage_id_pk" PRIMARY KEY("user_id","stage_id")
);
--> statement-breakpoint
CREATE TABLE "challenge_stages" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"challenge_id" uuid NOT NULL,
	"position" integer NOT NULL,
	"title" varchar(160) NOT NULL,
	"prompt_markdown" text DEFAULT '' NOT NULL,
	"hint" text DEFAULT '' NOT NULL,
	"flag" text DEFAULT '' NOT NULL,
	"flag_hmac" "bytea",
	CONSTRAINT "challenge_stages_challenge_id_position_unique" UNIQUE("challenge_id","position"),
	CONSTRAINT "challenge_stages_position_check" CHECK ("challenge_stages"."position">0)
);
--> statement-breakpoint
CREATE TABLE "challenges" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"title" varchar(160) NOT NULL,
	"summary" varchar(500) DEFAULT '' NOT NULL,
	"body_markdown" text DEFAULT '' NOT NULL,
	"category" varchar(60) DEFAULT '' NOT NULL,
	"difficulty" varchar(20) DEFAULT 'beginner' NOT NULL,
	"status" varchar(20) DEFAULT 'unpublished' NOT NULL,
	"target_template_id" uuid,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "challenges_title_check" CHECK (length(trim("challenges"."title"))>0),
	CONSTRAINT "challenges_status_check" CHECK ("challenges"."status" in ('unpublished','published')),
	CONSTRAINT "challenges_difficulty_check" CHECK ("challenges"."difficulty" in ('beginner','intermediate','advanced')),
	CONSTRAINT "challenges_lock_check" CHECK ("challenges"."lock_version">0)
);
--> statement-breakpoint
CREATE TABLE "target_instances" (
	"deployment_id" text NOT NULL,
	"expires_at" timestamp with time zone NOT NULL,
	"id" text PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"scope" text NOT NULL,
	"node_id" text NOT NULL,
	"boot_id" text NOT NULL,
	"state" text NOT NULL,
	"result" jsonb,
	"revision" integer DEFAULT 0 NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "target_instances_user_id_scope_unique" UNIQUE("user_id","scope"),
	CONSTRAINT "target_instances_deployment_check" CHECK ("target_instances"."deployment_id" ~ '^dep_[a-f0-9]{64}$')
);
--> statement-breakpoint
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
CREATE TABLE "module_images" (
	"module_id" uuid NOT NULL,
	"href" text NOT NULL,
	"asset_id" uuid NOT NULL,
	"public" boolean DEFAULT false NOT NULL,
	CONSTRAINT "module_images_module_id_href_pk" PRIMARY KEY("module_id","href")
);
--> statement-breakpoint
CREATE TABLE "lab_bundles" (
	"source" jsonb,
	"git_token_cipher" text,
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"actor_id" uuid NOT NULL,
	"hash" text NOT NULL,
	"manifest" jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "lab_preparations" (
	"attempt_id" uuid DEFAULT gen_random_uuid() NOT NULL,
	"mode" text DEFAULT 'cached' NOT NULL,
	"version" integer DEFAULT 1 NOT NULL,
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"template_id" uuid NOT NULL,
	"template_version" integer NOT NULL,
	"node_id" text NOT NULL,
	"boot_id" text NOT NULL,
	"state" text NOT NULL,
	"deployment_id" text,
	"summary" jsonb,
	"logs" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"error" text,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "lab_preparations_template_id_template_version_node_id_unique" UNIQUE("template_id","template_version","node_id")
);
--> statement-breakpoint
CREATE TABLE "notification_receipts" (
	"notification_id" uuid NOT NULL,
	"user_id" uuid NOT NULL,
	"read_at" timestamp with time zone,
	"dismissed_at" timestamp with time zone,
	CONSTRAINT "notification_receipts_notification_id_user_id_pk" PRIMARY KEY("notification_id","user_id")
);
--> statement-breakpoint
CREATE TABLE "notifications" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"kind" varchar(20) NOT NULL,
	"sender_id" uuid,
	"recipient_id" uuid,
	"title" varchar(120) NOT NULL,
	"body" text NOT NULL,
	"link" varchar(500),
	"published_at" timestamp with time zone DEFAULT now() NOT NULL,
	"withdrawn_at" timestamp with time zone,
	CONSTRAINT "notifications_recipient_check" CHECK (("notifications"."kind"='announcement' and "notifications"."recipient_id" is null) or ("notifications"."kind"='message' and "notifications"."recipient_id" is not null))
);
--> statement-breakpoint
CREATE TABLE "runtime_policies" (
	"id" varchar(16) PRIMARY KEY NOT NULL,
	"sandbox_ttl_minutes" integer NOT NULL,
	"sandbox_idle_minutes" integer NOT NULL,
	"target_ttl_minutes" integer NOT NULL,
	"lock_version" integer NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_by" uuid,
	CONSTRAINT "runtime_policy_singleton" CHECK ("runtime_policies"."id"='global'),
	CONSTRAINT "runtime_policy_limits" CHECK ("runtime_policies"."sandbox_ttl_minutes" between 1 and 1440 and "runtime_policies"."sandbox_idle_minutes" between 1 and "runtime_policies"."sandbox_ttl_minutes" and "runtime_policies"."target_ttl_minutes" between 1 and 1440 and "runtime_policies"."lock_version">0)
);
--> statement-breakpoint
CREATE TABLE "learn_home_settings" (
	"id" varchar(16) PRIMARY KEY NOT NULL,
	"config" jsonb NOT NULL,
	"lock_version" integer NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_by" uuid,
	CONSTRAINT "learn_home_singleton" CHECK ("learn_home_settings"."id"='global' and "learn_home_settings"."lock_version">0)
);
--> statement-breakpoint
CREATE TABLE "reference_documents" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"slug" varchar(100) NOT NULL,
	"title" varchar(160) NOT NULL,
	"summary" varchar(500) DEFAULT '' NOT NULL,
	"body_markdown" text NOT NULL,
	"status" varchar(20) DEFAULT 'unpublished' NOT NULL,
	"source_origin" text NOT NULL,
	"source_path" text NOT NULL,
	"source_commit" text NOT NULL,
	"manifest" jsonb NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"created_by" uuid NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"published_at" timestamp with time zone,
	CONSTRAINT "references_status_check" CHECK ("reference_documents"."status" in ('unpublished','published')),
	CONSTRAINT "references_version_check" CHECK ("reference_documents"."lock_version" > 0)
);
--> statement-breakpoint
CREATE TABLE "reference_images" (
	"reference_id" uuid NOT NULL,
	"href" text NOT NULL,
	"asset_id" uuid NOT NULL,
	CONSTRAINT "reference_images_reference_id_href_pk" PRIMARY KEY("reference_id","href")
);
--> statement-breakpoint
CREATE TABLE "auth_rate_limits" (
	"key" varchar(64) PRIMARY KEY NOT NULL,
	"attempts" integer NOT NULL,
	"window_at" timestamp with time zone NOT NULL,
	CONSTRAINT "auth_rate_limits_attempts_check" CHECK ("auth_rate_limits"."attempts" > 0)
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
ALTER TABLE "chapters" ADD CONSTRAINT "chapters_target_template_id_target_templates_id_fk" FOREIGN KEY ("target_template_id") REFERENCES "public"."target_templates"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_prerequisites" ADD CONSTRAINT "module_prerequisites_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_prerequisites" ADD CONSTRAINT "module_prerequisites_prerequisite_id_modules_id_fk" FOREIGN KEY ("prerequisite_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_default_target_template_id_target_templates_id_fk" FOREIGN KEY ("default_target_template_id") REFERENCES "public"."target_templates"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_category_id_content_categories_id_fk" FOREIGN KEY ("category_id") REFERENCES "public"."content_categories"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_cover_asset_id_assets_id_fk" FOREIGN KEY ("cover_asset_id") REFERENCES "public"."assets"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "modules" ADD CONSTRAINT "modules_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_progress" ADD CONSTRAINT "chapter_progress_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_progress" ADD CONSTRAINT "chapter_progress_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "question_completions" ADD CONSTRAINT "question_completions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "question_completions" ADD CONSTRAINT "question_completions_question_id_chapter_questions_id_fk" FOREIGN KEY ("question_id") REFERENCES "public"."chapter_questions"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "sandbox_identities" ADD CONSTRAINT "sandbox_identities_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "sandbox_node_settings" ADD CONSTRAINT "sandbox_node_settings_updated_by_users_id_fk" FOREIGN KEY ("updated_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "learning_route_modules" ADD CONSTRAINT "learning_route_modules_route_id_learning_routes_id_fk" FOREIGN KEY ("route_id") REFERENCES "public"."learning_routes"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "learning_route_modules" ADD CONSTRAINT "learning_route_modules_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "target_templates" ADD CONSTRAINT "target_templates_bundle_id_lab_bundles_id_fk" FOREIGN KEY ("bundle_id") REFERENCES "public"."lab_bundles"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_attempt_limits" ADD CONSTRAINT "challenge_attempt_limits_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_attempt_limits" ADD CONSTRAINT "challenge_attempt_limits_challenge_id_challenges_id_fk" FOREIGN KEY ("challenge_id") REFERENCES "public"."challenges"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_completions" ADD CONSTRAINT "challenge_completions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_completions" ADD CONSTRAINT "challenge_completions_stage_id_challenge_stages_id_fk" FOREIGN KEY ("stage_id") REFERENCES "public"."challenge_stages"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_stages" ADD CONSTRAINT "challenge_stages_challenge_id_challenges_id_fk" FOREIGN KEY ("challenge_id") REFERENCES "public"."challenges"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenges" ADD CONSTRAINT "challenges_target_template_id_target_templates_id_fk" FOREIGN KEY ("target_template_id") REFERENCES "public"."target_templates"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "target_instances" ADD CONSTRAINT "target_instances_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_sources" ADD CONSTRAINT "chapter_sources_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_sources" ADD CONSTRAINT "chapter_sources_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "content_previews" ADD CONSTRAINT "content_previews_actor_id_users_id_fk" FOREIGN KEY ("actor_id") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_sources" ADD CONSTRAINT "module_sources_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "question_sources" ADD CONSTRAINT "question_sources_question_id_chapter_questions_id_fk" FOREIGN KEY ("question_id") REFERENCES "public"."chapter_questions"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "question_sources" ADD CONSTRAINT "question_sources_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_images" ADD CONSTRAINT "module_images_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_images" ADD CONSTRAINT "module_images_asset_id_assets_id_fk" FOREIGN KEY ("asset_id") REFERENCES "public"."assets"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "lab_bundles" ADD CONSTRAINT "lab_bundles_actor_id_users_id_fk" FOREIGN KEY ("actor_id") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "lab_preparations" ADD CONSTRAINT "lab_preparations_template_id_target_templates_id_fk" FOREIGN KEY ("template_id") REFERENCES "public"."target_templates"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "notification_receipts" ADD CONSTRAINT "notification_receipts_notification_id_notifications_id_fk" FOREIGN KEY ("notification_id") REFERENCES "public"."notifications"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "notification_receipts" ADD CONSTRAINT "notification_receipts_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "notifications" ADD CONSTRAINT "notifications_sender_id_users_id_fk" FOREIGN KEY ("sender_id") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "notifications" ADD CONSTRAINT "notifications_recipient_id_users_id_fk" FOREIGN KEY ("recipient_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "runtime_policies" ADD CONSTRAINT "runtime_policies_updated_by_users_id_fk" FOREIGN KEY ("updated_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "learn_home_settings" ADD CONSTRAINT "learn_home_settings_updated_by_users_id_fk" FOREIGN KEY ("updated_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "reference_documents" ADD CONSTRAINT "reference_documents_created_by_users_id_fk" FOREIGN KEY ("created_by") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "reference_images" ADD CONSTRAINT "reference_images_reference_id_reference_documents_id_fk" FOREIGN KEY ("reference_id") REFERENCES "public"."reference_documents"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "reference_images" ADD CONSTRAINT "reference_images_asset_id_assets_id_fk" FOREIGN KEY ("asset_id") REFERENCES "public"."assets"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
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
CREATE UNIQUE INDEX "content_categories_name_unique" ON "content_categories" USING btree ("name");--> statement-breakpoint
CREATE INDEX "chapter_progress_user_visited_idx" ON "chapter_progress" USING btree ("user_id","last_visited_at");--> statement-breakpoint
CREATE UNIQUE INDEX "sandbox_identity_worker_id_idx" ON "sandbox_identities" USING btree ("worker_user_id");--> statement-breakpoint
CREATE INDEX "learning_route_modules_module_idx" ON "learning_route_modules" USING btree ("module_id");--> statement-breakpoint
CREATE INDEX "target_instances_expiry_idx" ON "target_instances" USING btree ("expires_at");--> statement-breakpoint
CREATE UNIQUE INDEX "chapter_sources_module_key_unique" ON "chapter_sources" USING btree ("module_id","match_key");--> statement-breakpoint
CREATE UNIQUE INDEX "module_sources_origin_path_unique" ON "module_sources" USING btree ("origin","source_path");--> statement-breakpoint
CREATE UNIQUE INDEX "question_sources_chapter_key_unique" ON "question_sources" USING btree ("chapter_id","match_key");--> statement-breakpoint
CREATE INDEX "lab_preparations_node_idx" ON "lab_preparations" USING btree ("node_id");--> statement-breakpoint
CREATE INDEX "notification_receipts_user_idx" ON "notification_receipts" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "notifications_published_idx" ON "notifications" USING btree ("published_at");--> statement-breakpoint
CREATE INDEX "notifications_recipient_idx" ON "notifications" USING btree ("recipient_id","published_at");--> statement-breakpoint
CREATE UNIQUE INDEX "references_slug_unique" ON "reference_documents" USING btree ("slug");--> statement-breakpoint
CREATE UNIQUE INDEX "references_source_unique" ON "reference_documents" USING btree ("source_origin","source_path");--> statement-breakpoint
CREATE INDEX "auth_rate_limits_window_idx" ON "auth_rate_limits" USING btree ("window_at");