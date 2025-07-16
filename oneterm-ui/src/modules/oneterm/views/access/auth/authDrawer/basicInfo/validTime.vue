<template>
  <a-form-model-item :label="$t('oneterm.auth.validTime')" prop="valid_from">
    <a-range-picker
      v-model="validTime"
      :show-time="{ format: 'HH:mm:ss' }"
      format="YYYY-MM-DD HH:mm:ss"
      style="width: 100%"
    />
  </a-form-model-item>
</template>

<script>
import moment from 'moment'

export default {
  name: 'ValidTime',
  props: {
    valid_from: {
      type: String,
      default: undefined
    },
    valid_to: {
      type: String,
      default: undefined
    }
  },
  computed: {
    validTime: {
      get() {
        if (this.valid_from && this.valid_to) {
          return [moment(this.valid_from), moment(this.valid_to)]
        }
        return []
      },
      set(val) {
        if (Array.isArray(val) && val.length === 2) {
          this.$emit('change', ['valid_from'], val[0] ? val[0].format('YYYY-MM-DD HH:mm:ss') : undefined)
          this.$emit('change', ['valid_to'], val[1] ? val[1].format('YYYY-MM-DD HH:mm:ss') : undefined)
        } else {
          this.$emit('change', ['valid_from'], undefined)
          this.$emit('change', ['valid_to'], undefined)
        }
        return val
      }
    }
  }
}
</script>
