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
ALTER TABLE "challenge_attempt_limits" ADD CONSTRAINT "challenge_attempt_limits_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_attempt_limits" ADD CONSTRAINT "challenge_attempt_limits_challenge_id_challenges_id_fk" FOREIGN KEY ("challenge_id") REFERENCES "public"."challenges"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_completions" ADD CONSTRAINT "challenge_completions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_completions" ADD CONSTRAINT "challenge_completions_stage_id_challenge_stages_id_fk" FOREIGN KEY ("stage_id") REFERENCES "public"."challenge_stages"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenge_stages" ADD CONSTRAINT "challenge_stages_challenge_id_challenges_id_fk" FOREIGN KEY ("challenge_id") REFERENCES "public"."challenges"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "challenges" ADD CONSTRAINT "challenges_target_template_id_target_templates_id_fk" FOREIGN KEY ("target_template_id") REFERENCES "public"."target_templates"("id") ON DELETE restrict ON UPDATE no action;