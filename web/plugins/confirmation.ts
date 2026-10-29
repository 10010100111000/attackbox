import { shallowRef } from 'vue'
import { createConfirmationController, type ConfirmationRequest } from '~/utils/confirmation'

export default defineNuxtPlugin(() => {
  const request = shallowRef<ConfirmationRequest | null>(null)
  const controller = createConfirmationController(value => {
    request.value = value
  })
  return { provide: { confirmation: { request, controller } } }
})
