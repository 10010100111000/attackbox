import {pgTable,uuid,text,integer,jsonb,timestamp,unique} from 'drizzle-orm/pg-core'
import {users} from './auth'
export const targetInstances=pgTable('target_instances',{
 id:text('id').primaryKey(),userId:uuid('user_id').notNull().references(()=>users.id,{onDelete:'restrict'}),scope:text('scope').notNull(),nodeId:text('node_id').notNull(),bootId:text('boot_id').notNull(),state:text('state').notNull(),image:text('image').notNull(),ports:jsonb('ports').$type<{port:number;protocol:'tcp'|'udp'}[]>().notNull(),result:jsonb('result').$type<Record<string,unknown>>(),revision:integer('revision').notNull().default(0),updatedAt:timestamp('updated_at',{withTimezone:true}).notNull().defaultNow(),
},t=>[unique().on(t.userId,t.scope)])
