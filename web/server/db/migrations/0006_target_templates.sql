CREATE TABLE "target_templates" (
	"id" uuid PRIMARY KEY DEFAULT gen_random_uuid() NOT NULL,
	"name" varchar(120) NOT NULL,
	"description" varchar(500) DEFAULT '' NOT NULL,
	"image" varchar(512) NOT NULL,
	"source_url" varchar(1000) DEFAULT '' NOT NULL,
	"ports" jsonb DEFAULT '[]'::jsonb NOT NULL,
	"access_instructions" text DEFAULT '' NOT NULL,
	"enabled" boolean DEFAULT true NOT NULL,
	"lock_version" integer DEFAULT 1 NOT NULL,
	"created_at" timestamp with time zone DEFAULT now() NOT NULL,
	"updated_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "target_templates_name_check" CHECK (length(trim("target_templates"."name"))>0),
	CONSTRAINT "target_templates_lock_check" CHECK ("target_templates"."lock_version">0),
	CONSTRAINT "target_templates_ports_check" CHECK (jsonb_typeof("target_templates"."ports")='array' and jsonb_array_length("target_templates"."ports")<=20)
);
