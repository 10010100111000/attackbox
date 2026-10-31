import { defineEventHandler, readBody } from 'h3'
import { completeRegistration } from '../../services/registration'

export default defineEventHandler(async event => {
  const body = await readBody(event)
  return completeRegistration(body?.token, body?.password)
})
