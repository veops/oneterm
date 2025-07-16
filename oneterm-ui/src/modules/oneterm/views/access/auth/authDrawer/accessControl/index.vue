<template>
  <div>
    <AuthorizationRole
      :form="form"
      @change="handleFormChange"
    />
    <a-form-model-item
      prop="permissions"
      :label="$t('oneterm.accessControl.permissionConfig')"
      :extra="$t('oneterm.accessControl.permissionConfigTip')"
    >
      <PermissionCheckbox
        :value="form.permissions"
        @change="(key, checked) => handleFormChange(['permissions', key], checked)"
      />
    </a-form-model-item>
    <AccessTime
      :accessTimeType="accessTimeType"
      :form="form"
      @update:accessTimeType="accessTimeType = $event"
      @change="handleFormChange"
    />
    <Command
      :form="form"
      @change="handleFormChange"
    />
    <IPWhiteList
      :form="form"
      @change="handleFormChange"
    />
  </div>
</template>

<script>
import { ACCESS_TIME_TYPE } from './constants.js'

import AuthorizationRole from './authorizationRole.vue'
import PermissionCheckbox from '@/modules/oneterm/views/systemSettings/accessControl/permissionCheckbox.vue'
import AccessTime from './accessTime.vue'
import Command from './command.vue'
import IPWhiteList from './ipWhiteList.vue'

export default {
  name: 'AccessControl',
  components: {
    AuthorizationRole,
    PermissionCheckbox,
    AccessTime,
    Command,
    IPWhiteList
  },
  props: {
    form: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      accessTimeType: ACCESS_TIME_TYPE.TIME_TEMPLATE
    }
  },
  methods: {
    initAccessTimeType(form) {
      let accessTimeType = ACCESS_TIME_TYPE.TIME_TEMPLATE
      if (
        form.access_control?.custom_time_ranges?.length &&
        form.access_control?.timezone
      ) {
        accessTimeType = ACCESS_TIME_TYPE.CUSTOM_TIME
      }

      this.accessTimeType = accessTimeType
    },
    getAccessTimeType() {
      return this.accessTimeType
    },
    handleFormChange(keys, value) {
      this.$emit(
        'change',
        {
          keys,
          value
        }
      )
    },
  }
}
</script>

<style lang="less" scoped>
</style>
