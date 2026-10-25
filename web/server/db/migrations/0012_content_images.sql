CREATE TABLE "module_images" (
	"module_id" uuid NOT NULL,
	"href" text NOT NULL,
	"asset_id" uuid NOT NULL,
	"public" boolean DEFAULT false NOT NULL,
	CONSTRAINT "module_images_module_id_href_pk" PRIMARY KEY("module_id","href")
);
--> statement-breakpoint
ALTER TABLE "module_images" ADD CONSTRAINT "module_images_module_id_modules_id_fk" FOREIGN KEY ("module_id") REFERENCES "public"."modules"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "module_images" ADD CONSTRAINT "module_images_asset_id_assets_id_fk" FOREIGN KEY ("asset_id") REFERENCES "public"."assets"("id") ON DELETE restrict ON UPDATE no action;