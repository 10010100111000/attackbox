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
ALTER TABLE "sandbox_node_settings" DROP CONSTRAINT "sandbox_node_limits_check";--> statement-breakpoint
ALTER TABLE "runtime_policies" ADD CONSTRAINT "runtime_policies_updated_by_users_id_fk" FOREIGN KEY ("updated_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "sandbox_node_settings" DROP COLUMN "ttl_seconds";--> statement-breakpoint
ALTER TABLE "sandbox_node_settings" DROP COLUMN "idle_seconds";--> statement-breakpoint
ALTER TABLE "sandbox_node_settings" ADD CONSTRAINT "sandbox_node_limits_check" CHECK ("sandbox_node_settings"."max_slots" between 1 and 250 and "sandbox_node_settings"."memory_mb" between 128 and 65536 and "sandbox_node_settings"."pids_max" between 32 and 4096);