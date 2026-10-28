import { defineEventHandler } from 'h3'
import { registrationDomains } from '../../utils/registration-policy'
export default defineEventHandler(() => {
  return { domains: registrationDomains() }
})
