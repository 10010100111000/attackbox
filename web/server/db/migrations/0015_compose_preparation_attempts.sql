ALTER TABLE "lab_preparations" ADD COLUMN "attempt_id" uuid DEFAULT gen_random_uuid() NOT NULL;--> statement-breakpoint
ALTER TABLE "lab_preparations" ADD COLUMN "mode" text DEFAULT 'cached' NOT NULL;--> statement-breakpoint
ALTER TABLE "lab_preparations" ADD COLUMN "version" integer DEFAULT 1 NOT NULL;