import {pgTable,uuid,varchar,text,boolean,jsonb,integer,timestamp,check} from 'drizzle-orm/pg-core'
import {sql} from 'drizzle-orm'
import type {TargetPort} from '../../../shared/target-templates'
import type {ComposeSettings} from '../../../shared/compose-labs'
export const targetTemplates=pgTable('target_templates',{

 runtime:text('runtime').notNull().default('container'),composeRevision:integer('compose_revision').notNull().default(0),bundleId:uuid('bundle_id'),composeSettings:jsonb('compose_settings').$type<ComposeSettings>(),variablesCipher:text('variables_cipher'),variableKeys:jsonb('variable_keys').$type<string[]>().notNull().default([]),
 id:uuid('id').primaryKey().defaultRandom(),name:varchar('name',{length:120}).notNull(),description:varchar('description',{length:500}).notNull().default(''),
 image:varchar('image',{length:512}).notNull(),sourceUrl:varchar('source_url',{length:1000}).notNull().default(''),
 ports:jsonb('ports').$type<TargetPort[]>().notNull().default([]),accessInstructions:text('access_instructions').notNull().default(''),
 enabled:boolean('enabled').notNull().default(true),lockVersion:integer('lock_version').notNull().default(1),
 createdAt:timestamp('created_at',{withTimezone:true}).notNull().defaultNow(),updatedAt:timestamp('updated_at',{withTimezone:true}).notNull().defaultNow(),
},t=>[check('target_templates_name_check',sql`length(trim(${t.name}))>0`),check('target_templates_lock_check',sql`${t.lockVersion}>0`),check('target_templates_ports_check',sql`jsonb_typeof(${t.ports})='array' and jsonb_array_length(${t.ports})<=20`)])
