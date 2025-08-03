<template>
  <div>
    <a-checkbox
      v-for="(item) in permissionCheckboxOptions"
      :key="item.value"
      :checked="value[item.value]"
      @change="(e) => $emit('change', item.value, e.target.checked)"
    >
      {{ $t(item.label) }}
      <a-tooltip v-if="item.value === PERMISSION_TYPE.COPY">
        <p slot="title">{{ $t('oneterm.accessControl.copyTip') }}</p>
        <a-icon type="info-circle" class="terminal-control-label-icon"/>
      </a-tooltip>
      <a-tooltip v-if="item.value === PERMISSION_TYPE.PASTE">
        <p slot="title">{{ $t('oneterm.accessControl.pasteTip') }}</p>
        <a-icon type="info-circle" class="terminal-control-label-icon"/>
      </a-tooltip>
    </a-checkbox>
  </div>
</template>

<script>
import { PERMISSION_TYPE, PERMISSION_TYPE_NAME } from './constants.js'

export default {
  name: 'PermissionCheckbox',
  props: {
    value: {
      type: Object,
      default: () => {}
    },
    hideOptions: {
      type: Array,
      default: () => []
    }
  },
  data() {
    return {
      PERMISSION_TYPE
    }
  },
  computed: {
    permissionCheckboxOptions() {
      const keys = Object.values(PERMISSION_TYPE).filter((key) => !this.hideOptions.includes(key))
      return keys.map((key) => ({
        value: key,
        label: PERMISSION_TYPE_NAME[key]
      }))
    }
  }
}
</script>

<style lang="less" scoped>
.ant-checkbox-wrapper + .ant-checkbox-wrapper {
  margin-left: 0px;
  margin-right: 8px;
}
</style>
