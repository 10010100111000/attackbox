import { defineEventHandler, getQuery } from 'h3'
import { listReferences } from '../../services/references'
export default defineEventHandler(async event => ({ data: await listReferences(getQuery(event).page) }))
