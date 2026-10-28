CREATE TABLE "feedback" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"user_id" uuid,
	"category" varchar(16) NOT NULL,
	"body" text NOT NULL,
	"source_path" varchar(500),
	"status" varchar(16) DEFAULT 'new' NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "feedback_category_check" CHECK ("feedback"."category" in ('bug', 'content', 'suggestion')),
	CONSTRAINT "feedback_status_check" CHECK ("feedback"."status" in ('new', 'in_progress', 'resolved')),
	CONSTRAINT "feedback_body_length_check" CHECK (length(btrim("feedback"."body")) between 1 and 4000)
);
--> statement-breakpoint
ALTER TABLE "feedback" ADD CONSTRAINT "feedback_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "feedback_status_created_idx" ON "feedback" USING btree ("status","created_at");--> statement-breakpoint
CREATE INDEX "feedback_user_created_idx" ON "feedback" USING btree ("user_id","created_at");