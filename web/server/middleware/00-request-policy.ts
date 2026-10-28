import { defineEventHandler } from 'h3'
import { guardApiRequest } from '../utils/request-policy'

export default defineEventHandler(guardApiRequest)
