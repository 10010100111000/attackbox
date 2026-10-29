<script setup lang="ts">
import { ref } from 'vue'
const text = ref('')
const amount = ref<number | string>(5)
const trimmed = ref('')
const deferred = ref('')
const checked = ref(false)
const disabled = ref(false)
const error = ref('')
const events = ref({ input: 0, change: 0, blur: 0, keydown: 0 })
const control = ref<{ focus: () => void } | null>(null)
</script>

<template>
  <form id="control-form" @submit.prevent>
    <label class="inline-row"
      >Inline input
      <UiInput
        ref="control"
        v-model="text"
        bare
        name="text"
        class="compact-input"
        required
        maxlength="10"
        aria-describedby="external-help"
        :error="error"
        @input="events.input++"
        @change="events.change++"
        @blur="events.blur++"
        @keydown="events.keydown++"
      />
    </label>
    <p id="external-help">External help</p>
    <label
      >Numeric input<UiInput
        v-model.number="amount"
        bare
        name="amount"
        type="number"
        min="1"
        max="10"
        required
    /></label>
    <label>Trimmed input<UiInput v-model.trim="trimmed" bare /></label>
    <label>Deferred input<UiInput v-model.lazy="deferred" bare /></label>
    <UiButton id="focus-input" @click="control?.focus()">Focus input</UiButton>
    <UiButton id="toggle-error" @click="error = error ? '' : 'Invalid'">Toggle error</UiButton>
    <label>Bare password<UiInput bare type="password" name="secret" /></label>
    <label for="enhanced-password">Enhanced password</label>
    <UiInput
      id="enhanced-password"
      type="password"
      :error="error"
      aria-describedby="external-help"
    />
    <UiButton id="disable-controls" @click="disabled = !disabled">Disable controls</UiButton>
    <fieldset :disabled="disabled">
      <label>Disabled input<UiInput bare /></label>
      <label class="checkbox-row"
        ><UiCheckbox v-model="checked" name="accepted" value="yes" required />Accept terms</label
      >
    </fieldset>
    <UiSeparator id="decorative-separator" decorative />
    <UiSeparator id="horizontal-separator" />
    <UiSeparator id="vertical-separator" orientation="vertical" :decorative="false" />
    <UiSeparator id="labeled-separator">Other methods</UiSeparator>
    <output id="control-values">{{
      JSON.stringify({ text, amount, trimmed, deferred, checked, events })
    }}</output>
  </form>
</template>

<style scoped>
.inline-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.compact-input {
  width: 160px;
}
.checkbox-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
