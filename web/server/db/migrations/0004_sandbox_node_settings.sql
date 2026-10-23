CREATE TABLE "sandbox_node_settings" (
	"node_id" varchar(64) PRIMARY KEY NOT NULL,
	"label" varchar(80) NOT NULL,
	"max_slots" integer NOT NULL,
	"ttl_seconds" integer NOT NULL,
	"idle_seconds" integer NOT NULL,
	"memory_mb" integer NOT NULL,
	"pids_max" integer NOT NULL,
	"draining" boolean DEFAULT false NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_by" uuid,
	CONSTRAINT "sandbox_node_limits_check" CHECK ("sandbox_node_settings"."max_slots" between 1 and 250 and "sandbox_node_settings"."ttl_seconds" between 60 and 86400 and "sandbox_node_settings"."idle_seconds" between 60 and "sandbox_node_settings"."ttl_seconds" and "sandbox_node_settings"."memory_mb" between 128 and 65536 and "sandbox_node_settings"."pids_max" between 32 and 4096),
	CONSTRAINT "sandbox_node_version_check" CHECK ("sandbox_node_settings"."lock_version" > 0),
	CONSTRAINT "sandbox_node_label_check" CHECK (length(btrim("sandbox_node_settings"."label")) > 0)
);
--> statement-breakpoint
ALTER TABLE "sandbox_node_settings" ADD CONSTRAINT "sandbox_node_settings_updated_by_users_id_fk" FOREIGN KEY ("updated_by") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;