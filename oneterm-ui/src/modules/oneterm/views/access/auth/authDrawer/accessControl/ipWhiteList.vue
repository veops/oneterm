<template>
  <a-form-model-item :label="$t('oneterm.auth.ipWhiteList')" prop="access_control.ip_whitelist">
    <a-select
      mode="tags"
      :value="form.access_control.ip_whitelist"
      :options="tagSelectOptions"
      :placeholder="$t('oneterm.auth.ipWhiteListTip')"
      @change="handleTagChange"
    />
  </a-form-model-item>
</template>

<script>
import _ from 'lodash'

export default {
  name: 'IPWhiteList',
  props: {
    form: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      tagSelectOptions: [],
    }
  },
  methods: {
    handleTagChange(value) {
      this.$emit('change', ['access_control', 'ip_whitelist'], value)
      const tagSelectOptions = this.tagSelectOptions.concat(value.map((item) => ({
        label: item,
        value: item
      })))
      this.tagSelectOptions = _.uniqBy(tagSelectOptions, 'value')
    }
  }
}
</script>
