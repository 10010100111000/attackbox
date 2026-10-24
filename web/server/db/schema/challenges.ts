import {pgTable,uuid,varchar,text,integer,timestamp,primaryKey,unique,check} from 'drizzle-orm/pg-core'
import {sql} from 'drizzle-orm'
import {users} from './auth'
import {targetTemplates} from './target-templates'
import {bytea} from './types'
export const challenges=pgTable('challenges',{
 id:uuid('id').primaryKey().defaultRandom(),title:varchar('title',{length:160}).notNull(),summary:varchar('summary',{length:500}).notNull().default(''),bodyMarkdown:text('body_markdown').notNull().default(''),category:varchar('category',{length:60}).notNull().default(''),difficulty:varchar('difficulty',{length:20}).notNull().default('beginner'),status:varchar('status',{length:20}).notNull().default('unpublished'),targetTemplateId:uuid('target_template_id').references(()=>targetTemplates.id,{onDelete:'restrict'}),lockVersion:integer('lock_version').notNull().default(1),createdAt:timestamp('created_at',{withTimezone:true}).notNull().defaultNow(),updatedAt:timestamp('updated_at',{withTimezone:true}).notNull().defaultNow(),
},t=>[check('challenges_title_check',sql`length(trim(${t.title}))>0`),check('challenges_status_check',sql`${t.status} in ('unpublished','published')`),check('challenges_difficulty_check',sql`${t.difficulty} in ('beginner','intermediate','advanced')`),check('challenges_lock_check',sql`${t.lockVersion}>0`)])
export const challengeStages=pgTable('challenge_stages',{
 id:uuid('id').primaryKey().defaultRandom(),challengeId:uuid('challenge_id').notNull().references(()=>challenges.id,{onDelete:'cascade'}),position:integer('position').notNull(),title:varchar('title',{length:160}).notNull(),promptMarkdown:text('prompt_markdown').notNull().default(''),hint:text('hint').notNull().default(''),flag:text('flag').notNull().default(''),flagHmac:bytea('flag_hmac'),
},t=>[unique().on(t.challengeId,t.position),check('challenge_stages_position_check',sql`${t.position}>0`)])
export const challengeCompletions=pgTable('challenge_completions',{
 userId:uuid('user_id').notNull().references(()=>users.id,{onDelete:'cascade'}),stageId:uuid('stage_id').notNull().references(()=>challengeStages.id,{onDelete:'cascade'}),passedAt:timestamp('passed_at',{withTimezone:true}).notNull().defaultNow(),
},t=>[primaryKey({columns:[t.userId,t.stageId]})])
export const challengeAttemptLimits=pgTable('challenge_attempt_limits',{
 userId:uuid('user_id').notNull().references(()=>users.id,{onDelete:'cascade'}),challengeId:uuid('challenge_id').notNull().references(()=>challenges.id,{onDelete:'cascade'}),windowAt:timestamp('window_at',{withTimezone:true}).notNull().defaultNow(),attempts:integer('attempts').notNull().default(0),
},t=>[primaryKey({columns:[t.userId,t.challengeId]})])
