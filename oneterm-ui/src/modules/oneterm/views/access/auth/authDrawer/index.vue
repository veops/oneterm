<template>
  <CustomDrawer
    width="850px"
    :visible="visible"
    :title="title"
    :maskClosable="false"
    @close="handleClose"
  >
    <a-form-model
      ref="authFormRef"
      :model="form"
      :rules="rules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 16 }"
      class="auth-form"
    >
      <div class="auth-form-title">{{ $t('oneterm.auth.basicInfo') }}</div>
      <BasicInfo
        :form="form"
        @change="handleFormChange"
      />

      <div class="auth-form-title">{{ $t('oneterm.auth.targetSelect') }}</div>
      <TargetSelect
        :form="form"
        @change="handleFormChange"
      />

      <div class="auth-form-title">{{ $t('oneterm.auth.accessControl') }}</div>
      <AccessControl
        :form="form"
        ref="accessControlRef"
        @change="handleFormChange"
      />

      <div class="custom-drawer-bottom-action">
        <a-button @click="handleClose">{{ $t('cancel') }}</a-button>
        <a-button @click="handleSubmit" type="primary">{{ $t('confirm') }}</a-button>
      </div>
    </a-form-model>
  </CustomDrawer>
</template>

<script>
import _ from 'lodash'
import momentTimezone from 'moment-timezone'
import { postAuth, putAuthById } from '@/modules/oneterm/api/authorizationV2.js'
import { getConfig } from '@/modules/oneterm/api/config'
import { TARGET_SELECT_TYPE } from '../constants.js'
import { ACCESS_TIME_TYPE } from './accessControl/constants.js'
import { PERMISSION_TYPE } from '@/modules/oneterm/views/systemSettings/accessControl/constants.js'

import BasicInfo from './basicInfo/index.vue'
import TargetSelect from './targetSelect/index.vue'
import AccessControl from './accessControl/index.vue'

const DEFAULT_PERMISSIONS = Object.values(PERMISSION_TYPE).reduce((config, key) => {
  config[key] = false
  return config
}, {})

const DEFAULT_FORM = {
  name: '',
  description: '',
  enabled: true,
  node_selector: {
    type: TARGET_SELECT_TYPE.ALL,
    values: [],
    exclude_ids: []
  },
  asset_selector: {
    type: TARGET_SELECT_TYPE.ALL,
    values: [],
    exclude_ids: []
  },
  account_selector: {
    type: TARGET_SELECT_TYPE.ALL,
    values: [],
    exclude_ids: []
  },
  rids: [],
  permissions: DEFAULT_PERMISSIONS,
  access_control: {
    time_template: {
      template_id: undefined
    },
    custom_time_ranges: [],
    timezone: momentTimezone.tz.guess(),
    cmd_ids: [],
    template_ids: [],
    ip_whitelist: [],
  },
  valid_from: undefined,
  valid_to: undefined
}

export default {
  name: 'AuthDrawer',
  components: {
    BasicInfo,
    TargetSelect,
    AccessControl
  },
  data() {
    return {
      visible: false,
      authId: '',
      form: _.cloneDeep(DEFAULT_FORM),
      rules: {
        name: [{ required: true, message: this.$t('placeholder1') }],
        rids: [{ required: true, message: this.$t('oneterm.auth.authorizationRoleTip') }],
        permissions: [{ required: true, message: this.$t('placeholder2') }]
      },
      confirmLoading: false
    }
  },
  computed: {
    title() {
      if (this.authId) {
        return this.$t('oneterm.auth.editAuth')
      }
      return this.$t('oneterm.auth.createAuth')
    }
  },
  mounted() {
    this.initDefaultPermissions()
  },
  methods: {
    initDefaultPermissions() {
      getConfig({
        info: true
      }).then((res) => {
        const default_permissions = res?.data?.default_permissions
        Object.keys(DEFAULT_FORM.permissions).forEach((key) => {
          DEFAULT_FORM.permissions[key] = default_permissions?.[key] ?? DEFAULT_FORM.permissions[key]
        })
        this.form.permissions = DEFAULT_FORM.permissions
      })
    },
    open(data) {
      this.visible = true

      if (data) {
        const editData = _.cloneDeep(data)

        // merge initialization form data (editData || DEFAULT_FORM)
        const form = {}
        Object.keys(DEFAULT_FORM).forEach((key) => {
          if (typeof DEFAULT_FORM[key] === 'object' && !Array.isArray(DEFAULT_FORM[key])) {
            form[key] = {}
            Object.keys(DEFAULT_FORM[key]).forEach((childKey) => {
              form[key][childKey] = editData?.[key]?.[childKey] ?? DEFAULT_FORM[key][childKey]
            })
          } else {
            form[key] = editData?.[key] ?? DEFAULT_FORM[key]
          }
        })

        this.form = form
        this.authId = editData?.id ?? ''
      }

      this.$nextTick(() => {
        this.$refs.accessControlRef.initAccessTimeType(this.form)
      })
    },
    /**
     * update form data
     * @param keys key list [root key, parent key, child key, ...]
     * @param value updated value
     */
    handleFormChange({
      keys,
      value,
    }) {
      let obj = this.form
      if (keys.length > 1) {
        obj = _.get(this.form, keys.slice(0, -1).join('.'))
      }
      this.$set(obj, keys.slice(-1), value)
    },
    handleClose() {
      this.form = _.cloneDeep(DEFAULT_FORM)
      this.authId = ''

      this.$refs.authFormRef.resetFields()
      this.visible = false
    },
    async handleSubmit() {
      this.$refs.authFormRef.validate(async (valid) => {
        if (!valid) return
        this.confirmLoading = true
        try {
          const { params, errorMessage } = this.handleSubmitParams()
          if (errorMessage) {
            this.$message.error(errorMessage)
            this.confirmLoading = false
            return
          }

          if (this.authId) {
            await putAuthById(this.authId, params)
            this.$message.success(this.$t('editSuccess'))
          } else {
            await postAuth(params)
            this.$message.success(this.$t('createSuccess'))
          }

          this.$emit('submit')
          this.handleClose()
        } catch (e) {
          console.error('submit error:', e)
        } finally {
          this.confirmLoading = false
        }
      })
    },
    handleSubmitParams() {
      const params = _.cloneDeep(this.form)
      const errorMessage = ''
      const accessTimeType = this.$refs.accessControlRef.getAccessTimeType()

      switch (accessTimeType) {
        case ACCESS_TIME_TYPE.TIME_TEMPLATE:
          params.access_control.custom_time_ranges = undefined
          params.access_control.timezone = undefined
          if (!params?.access_control?.time_template?.template_id) {
            params.access_control.time_template = undefined
          }
          break
        case ACCESS_TIME_TYPE.CUSTOM_TIME:
          params.access_control.time_template = undefined
          break
        default:
          break
      }

      return {
        params,
        errorMessage
      }
    }
  },
}
</script>

<style lang="less" scoped>
.auth-form {
  .auth-form-title {
    margin-bottom: 16px;
    font-weight: 600;
  }

  /deep/ .ant-input-number {
    width: 100%;
  }
}
</style>
