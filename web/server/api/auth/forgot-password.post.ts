import { defineEventHandler, readBody } from 'h3'
import { sendAccountEmail } from '../../services/account-email'
export default defineEventHandler(async (event) => {
  const body = await readBody(event)
  const email =
    typeof body?.email === 'string' ? body.email.trim().toLowerCase() : ''
  return sendAccountEmail(email, 'reset_password')
})
