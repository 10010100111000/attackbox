CREATE TABLE "learn_home_settings" (
	"id" varchar(16) PRIMARY KEY NOT NULL,
	"config" jsonb NOT NULL,
	"lock_version" integer NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_by" uuid,
	CONSTRAINT "learn_home_singleton" CHECK ("learn_home_settings"."id"='global' and "learn_home_settings"."lock_version">0)
);
--> statement-breakpoint
ALTER TABLE "learn_home_settings" ADD CONSTRAINT "learn_home_settings_updated_by_users_id_fk" FOREIGN KEY ("updated_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;