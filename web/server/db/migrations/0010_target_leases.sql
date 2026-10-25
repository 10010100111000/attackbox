ALTER TABLE "target_instances" ADD COLUMN "expires_at" timestamp with time zone NOT NULL DEFAULT now();
--> statement-breakpoint
ALTER TABLE "target_instances" ALTER COLUMN "expires_at" DROP DEFAULT;
--> statement-breakpoint
CREATE INDEX "target_instances_expiry_idx" ON "target_instances" ("expires_at");
