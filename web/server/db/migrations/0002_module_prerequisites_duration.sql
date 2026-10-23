CREATE TABLE "module_prerequisites" (
	"module_id" uuid NOT NULL,
	"prerequisite_id" uuid NOT NULL,
	"position" integer NOT NULL,
	CONSTRAINT "module_prerequisites_module_id_prerequisite_id_pk" PRIMARY KEY("module_id","prerequisite_id"),
	CONSTRAINT "module_prerequisites_self_check" CHECK ("module_prerequisites"."module_id" <> "module_prerequisites"."prerequisite_id"),
	CONSTRAINT "module_prerequisites_position_check" CHECK ("module_prerequisites"."position" > 0)
);
--> statement-breakpoint
ALTER TABLE "chapters" ADD COLUMN "duration_minutes" integer;--> statement-breakpoint
ALTER TABLE "module_prerequisites" ADD CONSTRAINT "module_prerequisites_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_prerequisites" ADD CONSTRAINT "module_prerequisites_prerequisite_id_modules_id_fk" FOREIGN KEY ("prerequisite_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "module_prerequisites_position_unique" ON "module_prerequisites" USING btree ("module_id","position");--> statement-breakpoint
ALTER TABLE "chapters" ADD CONSTRAINT "chapters_duration_check" CHECK ("chapters"."duration_minutes" is null or "chapters"."duration_minutes" between 1 and 1440);