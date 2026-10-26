import {pgTable,uuid,text,integer,jsonb,timestamp,unique,index} from 'drizzle-orm/pg-core'
import {users} from './auth'
export const targetInstances=pgTable('target_instances',{
 runtime:text('runtime').notNull().default('container'),deploymentId:text('deployment_id'),
 expiresAt:timestamp('expires_at',{withTimezone:true}).notNull(),
 id:text('id').primaryKey(),userId:uuid('user_id').notNull().references(()=>users.id,{onDelete:'restrict'}),scope:text('scope').notNull(),nodeId:text('node_id').notNull(),bootId:text('boot_id').notNull(),state:text('state').notNull(),image:text('image').notNull(),ports:jsonb('ports').$type<{port:number;protocol:'tcp'|'udp'}[]>().notNull(),result:jsonb('result').$type<Record<string,unknown>>(),revision:integer('revision').notNull().default(0),updatedAt:timestamp('updated_at',{withTimezone:true}).notNull().defaultNow(),
},t=>[index('target_instances_expiry_idx').on(t.expiresAt),unique().on(t.userId,t.scope)])
