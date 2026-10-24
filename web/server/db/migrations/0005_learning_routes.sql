CREATE TABLE "learning_route_modules" (
	"route_id" uuid NOT NULL,
	"module_id" uuid NOT NULL,
	"position" integer NOT NULL,
	CONSTRAINT "learning_route_modules_route_id_module_id_pk" PRIMARY KEY("route_id","module_id"),
	CONSTRAINT "learning_route_modules_route_id_position_unique" UNIQUE("route_id","position"),
	CONSTRAINT "learning_route_modules_position_check" CHECK ("learning_route_modules"."position">0)
);
--> statement-breakpoint
CREATE TABLE "learning_routes" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"title" varchar(160) NOT NULL,
	"summary" varchar(500) DEFAULT '' NOT NULL,
	"status" varchar(20) DEFAULT 'unpublished' NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "learning_routes_title_check" CHECK (length(trim("learning_routes"."title"))>0),
	CONSTRAINT "learning_routes_status_check" CHECK ("learning_routes"."status" in ('unpublished','published')),
	CONSTRAINT "learning_routes_lock_version_check" CHECK ("learning_routes"."lock_version">0)
);
--> statement-breakpoint
ALTER TABLE "learning_route_modules" ADD CONSTRAINT "learning_route_modules_route_id_learning_routes_id_fk" FOREIGN KEY ("route_id") REFERENCES "public"."learning_routes"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "learning_route_modules" ADD CONSTRAINT "learning_route_modules_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE restrict ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "learning_route_modules_module_idx" ON "learning_route_modules" USING btree ("module_id");