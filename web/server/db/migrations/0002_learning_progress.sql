CREATE TABLE "chapter_progress" (
	"user_id" uuid NOT NULL,
	"chapter_id" uuid NOT NULL,
	"started_at" timestamp with time zone DEFAULT now() NOT NULL,
	"last_visited_at" timestamp with time zone DEFAULT now() NOT NULL,
	"completed_at" timestamp with time zone,
	CONSTRAINT "chapter_progress_user_id_chapter_id_pk" PRIMARY KEY("user_id","chapter_id")
);
--> statement-breakpoint
CREATE TABLE "question_completions" (
	"user_id" uuid NOT NULL,
	"question_id" uuid NOT NULL,
	"passed_at" timestamp with time zone DEFAULT now() NOT NULL,
	CONSTRAINT "question_completions_user_id_question_id_pk" PRIMARY KEY("user_id","question_id")
);
--> statement-breakpoint
ALTER TABLE "chapter_progress" ADD CONSTRAINT "chapter_progress_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "chapter_progress" ADD CONSTRAINT "chapter_progress_chapter_id_chapters_id_fk" FOREIGN KEY ("chapter_id") REFERENCES "public"."chapters"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "question_completions" ADD CONSTRAINT "question_completions_user_id_users_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."users"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "question_completions" ADD CONSTRAINT "question_completions_question_id_chapter_questions_id_fk" FOREIGN KEY ("question_id") REFERENCES "public"."chapter_questions"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "chapter_progress_user_visited_idx" ON "chapter_progress" USING btree ("user_id","last_visited_at");