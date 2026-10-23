CREATE TABLE "sandbox_identities" (
	"user_id" uuid PRIMARY KEY NOT NULL,
	"worker_user_id" bigserial NOT NULL,
	CONSTRAINT "sandbox_identity_safe_integer_check" CHECK ("sandbox_identities"."worker_user_id" > 0 and "sandbox_identities"."worker_user_id" <= 9007199254740991)
);
--> statement-breakpoint
ALTER TABLE "sandbox_identities" ADD CONSTRAINT "sandbox_identities_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE UNIQUE INDEX "sandbox_identity_worker_id_idx" ON "sandbox_identities" USING btree ("worker_user_id");