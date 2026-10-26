import {pgTable,uuid,text,integer,jsonb,timestamp,unique,index} from 'drizzle-orm/pg-core'
import {users} from './auth'
import {targetTemplates} from './target-templates'
import type {ComposeSummary,LabGitSource} from '../../../shared/compose-labs'
export const labBundles=pgTable('lab_bundles',{
 source:jsonb('source').$type<Omit<LabGitSource,'hasToken'>>(),gitTokenCipher:text('git_token_cipher'),
 id:uuid('id').primaryKey().defaultRandom(),actorId:uuid('actor_id').notNull().references(()=>users.id,{onDelete:'restrict'}),hash:text('hash').notNull(),manifest:jsonb('manifest').$type<Record<string,string>>().notNull(),createdAt:timestamp('created_at',{withTimezone:true}).notNull().defaultNow(),
})
export const labPreparations=pgTable('lab_preparations',{
 id:uuid('id').primaryKey().defaultRandom(),templateId:uuid('template_id').notNull().references(()=>targetTemplates.id,{onDelete:'cascade'}),templateVersion:integer('template_version').notNull(),nodeId:text('node_id').notNull(),bootId:text('boot_id').notNull(),state:text('state').notNull(),deploymentId:text('deployment_id'),summary:jsonb('summary').$type<ComposeSummary>(),logs:jsonb('logs').$type<string[]>().notNull().default([]),error:text('error'),updatedAt:timestamp('updated_at',{withTimezone:true}).notNull().defaultNow(),
},t=>[unique().on(t.templateId,t.templateVersion,t.nodeId),index('lab_preparations_node_idx').on(t.nodeId)])
