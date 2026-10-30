CREATE TABLE "ad_events" (
	"visit_id" uuid NOT NULL,
	"ad_id" uuid NOT NULL,
	"ad_name" varchar(120) NOT NULL,
	"placement" varchar(32) NOT NULL,
	"kind" varchar(16) NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "ad_events_visit_id_ad_id_placement_kind_pk" PRIMARY KEY("visit_id","ad_id","placement","kind"),
	CONSTRAINT "ad_events_kind_check" CHECK ("ad_events"."kind" in ('impression','click'))
);
--> statement-breakpoint
CREATE TABLE "advertisements" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"name" varchar(120) NOT NULL,
	"title" varchar(120) NOT NULL,
	"description" varchar(300) DEFAULT '' NOT NULL,
	"image_url" text DEFAULT '' NOT NULL,
	"href" text NOT NULL,
	"cta" varchar(30) DEFAULT '' NOT NULL,
	"sponsor" varchar(80) DEFAULT '' NOT NULL,
	"placements" text[] NOT NULL,
	"enabled" boolean DEFAULT false NOT NULL,
	"weight" integer DEFAULT 1 NOT NULL,
	"starts_at" timestamp with time zone,
	"ends_at" timestamp with time zone,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "advertisements_weight_check" CHECK ("advertisements"."weight" between 1 and 100),
	CONSTRAINT "advertisements_version_check" CHECK ("advertisements"."lock_version" > 0),
	CONSTRAINT "advertisements_window_check" CHECK ("advertisements"."starts_at" is null or "advertisements"."ends_at" is null or "advertisements"."starts_at" < "advertisements"."ends_at"),
	CONSTRAINT "advertisements_placements_check" CHECK (cardinality("advertisements"."placements") between 1 and 2 and "advertisements"."placements" <@ array['learn_catalog','reference_footer']::text[])
);
--> statement-breakpoint
CREATE TABLE "analytics_budgets" (
	"key" varchar(64) PRIMARY KEY NOT NULL,
	"attempts" integer NOT NULL,
	"window_at" timestamp with time zone DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "analytics_visits" (
	"id" uuid PRIMARY KEY NOT NULL,
	"visitor_key" varchar(64) NOT NULL,
	"path" varchar(500) NOT NULL,
	"source" varchar(253) NOT NULL,
	"medium" varchar(80) NOT NULL,
	"campaign" varchar(120) NOT NULL,
	"channel" varchar(16) NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"read_at" timestamp with time zone
);
--> statement-breakpoint
ALTER TABLE "ad_events" ADD CONSTRAINT "ad_events_visit_id_analytics_visits_id_fk" FOREIGN KEY ("visit_id") REFERENCES "public"."analytics_visits"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "ad_events_ad_created_idx" ON "ad_events" USING btree ("ad_id","created_at");--> statement-breakpoint
CREATE INDEX "analytics_budgets_window_idx" ON "analytics_budgets" USING btree ("window_at");--> statement-breakpoint
CREATE INDEX "analytics_visits_created_idx" ON "analytics_visits" USING btree ("created_at");--> statement-breakpoint
CREATE INDEX "analytics_visits_visitor_idx" ON "analytics_visits" USING btree ("visitor_key","created_at");