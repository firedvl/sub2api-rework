<template>
  <span class="payment-method-mark">
    <img :src="icon" alt="" aria-hidden="true" :style="{ width: `${size}px`, height: `${size}px` }" />
    <span v-if="!iconOnly" class="truncate">{{ label }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { isBuiltInAlipayMethod, isBuiltInWxpayMethod } from '@/components/payment/providerConfig'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import stripeIcon from '@/assets/icons/stripe.svg'
import airwallexIcon from '@/assets/icons/airwallex.svg'
import paymentIcon from '@/assets/icons/payment.svg'

/** Payment method logo (the checkout's own icons) with its localized name. */
const props = withDefaults(defineProps<{
  type: string
  size?: number
  iconOnly?: boolean
}>(), { size: 18, iconOnly: false })

const { t } = useI18n()

const icon = computed(() => {
  if (isBuiltInAlipayMethod(props.type)) return alipayIcon
  if (isBuiltInWxpayMethod(props.type)) return wxpayIcon
  if (props.type === 'stripe') return stripeIcon
  if (props.type === 'airwallex') return airwallexIcon
  return paymentIcon
})

const label = computed(() => t('payment.methods.' + props.type, props.type))
</script>

<style scoped>
.payment-method-mark {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
}

.payment-method-mark img {
  flex: none;
  object-fit: contain;
}
</style>
