import { defineEventHandler, setResponseHeader } from 'h3'
import { registrationDomains } from '../../utils/registration-policy'
export default defineEventHandler(event => {
  setResponseHeader(event, 'Cache-Control', 'no-store')
  return { domains: registrationDomains() }
})
