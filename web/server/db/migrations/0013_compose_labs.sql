CREATE TABLE "lab_bundles" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"actor_id" uuid NOT NULL,
	"hash" text NOT NULL,
	"manifest" jsonb NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "lab_preparations" (
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
ALTER TABLE "target_templates" ADD COLUMN "runtime" text DEFAULT 'container' NOT NULL;--> statement-breakpoint
ALTER TABLE "target_templates" ADD COLUMN "compose_revision" integer DEFAULT 0 NOT NULL;--> statement-breakpoint
ALTER TABLE "target_templates" ADD COLUMN "bundle_id" uuid;--> statement-breakpoint
ALTER TABLE "target_templates" ADD COLUMN "compose_settings" jsonb;--> statement-breakpoint
ALTER TABLE "target_templates" ADD COLUMN "variables_cipher" text;--> statement-breakpoint
ALTER TABLE "target_templates" ADD COLUMN "variable_keys" jsonb DEFAULT '[]'::jsonb NOT NULL;--> statement-breakpoint
ALTER TABLE "target_instances" ADD COLUMN "runtime" text DEFAULT 'container' NOT NULL;--> statement-breakpoint
ALTER TABLE "target_instances" ADD COLUMN "deployment_id" text;--> statement-breakpoint
ALTER TABLE "lab_bundles" ADD CONSTRAINT "lab_bundles_actor_id_users_id_fk" FOREIGN KEY ("actor_id") REFERENCES "public"."users"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "lab_preparations" ADD CONSTRAINT "lab_preparations_template_id_target_templates_id_fk" FOREIGN KEY ("template_id") REFERENCES "public"."target_templates"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "lab_preparations_node_idx" ON "lab_preparations" USING btree ("node_id");