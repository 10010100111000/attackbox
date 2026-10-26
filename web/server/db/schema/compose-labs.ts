import {pgTable,uuid,text,integer,jsonb,timestamp,unique,index} from 'drizzle-orm/pg-core'
import {targetTemplates} from './target-templates'
import type {ComposeSummary} from '../../../shared/compose-labs'
export {labBundles} from './lab-bundles'
export const labPreparations=pgTable('lab_preparations',{
 attemptId:uuid('attempt_id').notNull().defaultRandom(),mode:text('mode').notNull().default('cached'),version:integer('version').notNull().default(1),
 id:uuid('id').primaryKey().defaultRandom(),templateId:uuid('template_id').notNull().references(()=>targetTemplates.id,{onDelete:'cascade'}),templateVersion:integer('template_version').notNull(),nodeId:text('node_id').notNull(),bootId:text('boot_id').notNull(),state:text('state').notNull(),deploymentId:text('deployment_id'),summary:jsonb('summary').$type<ComposeSummary>(),logs:jsonb('logs').$type<string[]>().notNull().default([]),error:text('error'),updatedAt:timestamp('updated_at',{withTimezone:true}).notNull().defaultNow(),
},t=>[unique().on(t.templateId,t.templateVersion,t.nodeId),index('lab_preparations_node_idx').on(t.nodeId)])
