-- Position uniqueness is checked at commit so retained records can swap slots.
DROP TABLE "content_previews";--> statement-breakpoint
DROP INDEX "chapter_questions_position_unique";--> statement-breakpoint
DROP INDEX "chapters_module_position_unique";--> statement-breakpoint
ALTER TABLE "chapter_questions" ADD CONSTRAINT "chapter_questions_position_unique" UNIQUE("chapter_id","position") DEFERRABLE INITIALLY DEFERRED;--> statement-breakpoint
ALTER TABLE "chapters" ADD CONSTRAINT "chapters_module_position_unique" UNIQUE("module_id","position") DEFERRABLE INITIALLY DEFERRED;
--> statement-breakpoint
ALTER TABLE "challenge_stages" DROP CONSTRAINT "challenge_stages_challenge_id_position_unique";
--> statement-breakpoint
ALTER TABLE "challenge_stages" ADD CONSTRAINT "challenge_stages_challenge_id_position_unique" UNIQUE("challenge_id","position") DEFERRABLE INITIALLY DEFERRED;
