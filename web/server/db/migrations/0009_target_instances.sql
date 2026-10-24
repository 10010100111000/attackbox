CREATE TABLE "target_instances" (
	"id" text PRIMARY KEY NOT NULL,
	"user_id" uuid NOT NULL,
	"scope" text NOT NULL,
	"node_id" text NOT NULL,
	"boot_id" text NOT NULL,
	"state" text NOT NULL,
	"image" text NOT NULL,
	"ports" jsonb NOT NULL,
	"result" jsonb,
	"revision" integer DEFAULT 0 NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "target_instances_user_id_scope_unique" UNIQUE("user_id","scope")
);
--> statement-breakpoint
ALTER TABLE "target_instances" ADD CONSTRAINT "target_instances_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;