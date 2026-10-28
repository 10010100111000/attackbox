ALTER TABLE "chapter_questions" DROP CONSTRAINT "chapter_questions_hmac_check";--> statement-breakpoint
ALTER TABLE "chapter_questions" DROP CONSTRAINT "chapter_questions_key_version_check";--> statement-breakpoint
ALTER TABLE "chapter_questions" DROP COLUMN "answer_hmac";--> statement-breakpoint
ALTER TABLE "chapter_questions" DROP COLUMN "answer_key_version";--> statement-breakpoint
ALTER TABLE "challenge_stages" DROP COLUMN "flag_hmac";
