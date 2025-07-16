<template>
  <CustomDrawer
    :title="title"
    :visible="visible"
    :width="600"
    @close="handleCancel"
  >
    <a-form-model
      ref="storageConfigFormRef"
      class="storage-config-form"
      :model="form"
      :rules="dynamicRules"
      :label-col="{ span: 7 }"
      :wrapper-col="{ span: 15 }"
    >
      <div class="storage-config-form-title">
        {{ $t('oneterm.storageConfig.basicConfig') }}
      </div>
      <a-form-model-item :label="$t('name')" prop="name">
        <a-input v-model="form.name" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <a-form-model-item :label="$t('type')" prop="type">
        <a-select
          :value="form.type"
          :options="configTypeSelectOptions"
          @change="handleConfigTypeChange"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('description')" prop="description">
        <a-input v-model="form.description" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.storageConfig.priority')" prop="priority">
        <a-input-number
          v-model="form.priority"
          :placeholder="$t('placeholder1')"
          :min="1"
          :precision="0"
          :step="1"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.storageConfig.isPrimary')" prop="is_primary">
        <a-switch v-model="form.is_primary" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.isEnable')" prop="enabled">
        <a-switch v-model="form.enabled" />
      </a-form-model-item>

      <div class="storage-config-form-title">
        {{ $t('oneterm.storageConfig.advancedConfig') }}
        <a-tooltip :title="$t('oneterm.storageConfig.advancedConfigTip')">
          <a-icon
            type="info-circle"
            class="storage-config-form-title-icon"
          />
        </a-tooltip>
      </div>
      <a-form-model-item
        v-for="(item) in dynamicConfigForm"
        :key="item.field"
        :label="$t(item.label)"
        :prop="'config.' + item.field"
      >
        <components
          :is="item.component"
          v-model="form.config[item.field]"
          v-bind="item.componentProps"
        />
      </a-form-model-item>
    </a-form-model>

    <div class="custom-drawer-bottom-action">
      <a-button :loading="confirmLoading" @click="handleCancel">
        {{ $t('cancel') }}
      </a-button>
      <a-button
        v-if="configId"
        type="primary"
        class="ops-button-ghost"
        ghost
        :loading="confirmLoading"
        @click="handleTestConnection"
      >
        {{ $t('oneterm.storageConfig.testConnection') }}
      </a-button>
      <a-button
        type="primary"
        :loading="confirmLoading"
        @click="handleOk"
      >
        {{ $t('confirm') }}
      </a-button>
    </div>
  </CustomDrawer>
</template>

<script>
import _ from 'lodash'
import {
  STORAGE_CONFIG_TYPE,
  configTypeSelectOptions,
  LOCAL_CONFIG_FORM,
  MIN_IO_CONFIG_FORM,
  S3_CONFIG_FORM,
  OSS_CONFIG_FORM,
  COS_CONFIG_FORM,
  AZURE_CONFIG_FORM,
  OBS_CONFIG_FORM,
  OOS_CONFIG_FORM
} from './constants.js'
import {
  postStorageConfigs,
  putStorageConfigs,
  testConnection
} from '@/modules/oneterm/api/storage.js'

export default {
  name: 'ConfigDrawer',
  data() {
    return {
      visible: false,
      confirmLoading: false,
      configId: '',

      form: {
        name: '',
        type: STORAGE_CONFIG_TYPE.LOCAL,
        description: '',
        priority: 1,
        is_primary: false,
        enabled: false,
        config: {}
      },
      configTypeSelectOptions: [
        ...configTypeSelectOptions
      ],
      LOCAL_CONFIG_FORM
    }
  },
  computed: {
    title() {
      if (this.configId) {
        return this.$t('oneterm.storageConfig.editConfig')
      }
      return this.$t('oneterm.storageConfig.createConfig')
    },
    dynamicConfigForm() {
      switch (this.form.type) {
        case STORAGE_CONFIG_TYPE.LOCAL:
          return LOCAL_CONFIG_FORM
        case STORAGE_CONFIG_TYPE.MIN_IO:
          return MIN_IO_CONFIG_FORM
        case STORAGE_CONFIG_TYPE.S3:
          return S3_CONFIG_FORM
        case STORAGE_CONFIG_TYPE.OSS:
          return OSS_CONFIG_FORM
        case STORAGE_CONFIG_TYPE.COS:
          return COS_CONFIG_FORM
        case STORAGE_CONFIG_TYPE.AZURE:
          return AZURE_CONFIG_FORM
        case STORAGE_CONFIG_TYPE.OBS:
          return OBS_CONFIG_FORM
        case STORAGE_CONFIG_TYPE.OOS:
          return OOS_CONFIG_FORM
        default:
          return []
      }
    },
    dynamicRules() {
      const baseRules = {
        name: [{ required: true, message: this.$t('placeholder1') }],
        type: [{ required: true, message: this.$t('placeholder2') }],
        priority: [{ required: true, message: this.$t('placeholder2') }]
      }

      const configRules = {}
      this.dynamicConfigForm.forEach(item => {
        if (item?.required) {
          configRules[`config.${item.field}`] = [
            { required: true, message: this.$t('placeholder1') }
          ]
        }
      })
      return {
        ...baseRules,
        ...configRules
      }
    }
  },
  methods: {
    open(data) {
      const form = Object.keys(_.omit(this.form, 'config')).reduce((form, key) => {
        form[key] = data?.[key] ?? this.form[key]
        return form
      }, {})
      const rawConfig = data?.config || {}
      const config = {}
      Object.keys(rawConfig).forEach(key => {
        config[key] = this.parseConfigValue(rawConfig[key])
      })
      form.config = this.handleConfigData(form.type, config)
      this.form = form

      this.configId = data?.id || ''
      this.visible = true
    },

    // string type => form type
    parseConfigValue(val) {
      if (val === 'true') return true
      if (val === 'false') return false
      if (typeof val === 'string' && val !== '' && !isNaN(val)) return Number(val)
      return val
    },

    // form type => string type
    stringifyConfigValue(val) {
      if (typeof val === 'boolean') return val ? 'true' : 'false'
      if (typeof val === 'number') return String(val)
      return val == null ? '' : String(val)
    },

    handleConfigData(type, config) {
      const COMMON_FIELDS = ['retention_days', 'archive_days', 'cleanup_enabled', 'archive_enabled']

      const CONFIG_FIELDS_MAP = {
        [STORAGE_CONFIG_TYPE.LOCAL]: ['base_path', 'path_strategy'],
        [STORAGE_CONFIG_TYPE.MIN_IO]: [
          'endpoint', 'access_key_id', 'secret_access_key', 'bucket_name', 'use_ssl', 'path_strategy'
        ],
        [STORAGE_CONFIG_TYPE.S3]: [
          'region', 'access_key_id', 'secret_access_key', 'bucket_name', 'endpoint', 'use_ssl', 'path_strategy'
        ],
        [STORAGE_CONFIG_TYPE.OSS]: [
          'endpoint', 'access_key_id', 'access_key_secret', 'bucket_name', 'path_strategy'
        ],
        [STORAGE_CONFIG_TYPE.COS]: [
          'region', 'secret_id', 'secret_key', 'bucket_name', 'app_id', 'path_strategy'
        ],
        [STORAGE_CONFIG_TYPE.AZURE]: [
          'account_name', 'account_key', 'container_name', 'endpoint_suffix', 'path_strategy'
        ],
        [STORAGE_CONFIG_TYPE.OBS]: [
          'endpoint', 'access_key_id', 'secret_access_key', 'bucket_name', 'path_strategy'
        ],
        [STORAGE_CONFIG_TYPE.OOS]: [
          'endpoint', 'access_key_id', 'secret_access_key', 'bucket_name', 'region', 'path_strategy'
        ]
      }

      const defaultValue = {
        use_ssl: false,
        path_strategy: 'date_hierarchy',
        retention_days: 30,
        archive_days: 30,
        cleanup_enabled: true,
        archive_enabled: true
      }

      const fields = CONFIG_FIELDS_MAP[type] || []
      fields.push(...COMMON_FIELDS)

      const newConfig = {}
      fields.forEach((key) => {
        newConfig[key] = config?.[key] ?? defaultValue?.[key] ?? ''
      })

      return newConfig
    },
    handleConfigTypeChange(type) {
      if (this.form.type !== type) {
        this.form.type = type
        this.form.config = this.handleConfigData(type)
      }
    },
    handleCancel() {
      this.form = {
        name: '',
        type: STORAGE_CONFIG_TYPE.LOCAL,
        description: '',
        priority: 1,
        is_primary: false,
        enabled: false,
        config: {}
      }
      this.configId = ''
      this.confirmLoading = false
      this.$refs.storageConfigFormRef.resetFields()
      this.visible = false
    },
    async handleTestConnection() {
      const submitForm = await this.validForm()
      if (!submitForm) {
        return
      }

      this.confirmLoading = true
      testConnection(submitForm)
        .then((res) => {
          this.$message.success(this.$t('oneterm.storageConfig.connectionSuccess'))
        })
        .finally(() => {
          this.confirmLoading = false
        })
    },
    async handleOk() {
      const submitForm = await this.validForm()
      if (!submitForm) {
        return
      }

      this.confirmLoading = true
      try {
        if (this.configId) {
          await putStorageConfigs(
            this.configId,
            submitForm
          )
          this.$message.success(this.$t('editSuccess'))
        } else {
          const res = await postStorageConfigs(submitForm)
          this.configId = res?.data?.id || ''
          this.$message.success(this.$t('createSuccess'))
        }
        this.$emit('ok')
      } finally {
        this.confirmLoading = false
      }
    },
    validForm() {
      return new Promise((resolve) => {
        this.$refs.storageConfigFormRef.validate(async (valid) => {
          let submitForm = null
          if (valid) {
            const config = {}
            Object.keys(this.form.config).forEach(key => {
              config[key] = this.stringifyConfigValue(this.form.config[key])
            })
            submitForm = {
              ...this.form,
              config
            }
          }

          resolve(submitForm)
        })
      })
    }
  }
}
</script>

<style lang="less" scoped>
.storage-config-form {
  &-title {
    display: flex;
    margin-bottom: 18px;
    font-size: 14px;
    font-weight: 600;
    align-items: center;

    &-icon {
      color: @text-color_3;
      margin-left: 6px;
      cursor: pointer;
    }
  }

  /deep/ .ant-input-number {
    width: 100%;
  }
}
</style>
