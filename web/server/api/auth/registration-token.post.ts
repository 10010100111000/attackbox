import { defineEventHandler, readBody } from 'h3'
import { inspectRegistration } from '../../services/registration'

export default defineEventHandler(async event => {
  const body = await readBody(event)
  return inspectRegistration(body?.token)
})
