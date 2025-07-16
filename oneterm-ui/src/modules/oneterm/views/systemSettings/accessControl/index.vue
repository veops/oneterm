<template>
  <div class="access-control">
    <a-form-model ref="configForm" :model="form" :rules="rules" :label-col="{ span: 7 }" :wrapper-col="{ span: 14 }">
      <a-form-model-item :label="$t('oneterm.accessControl.timeout')" prop="timeout">
        <a-input-number
          :min="0"
          :max="7200"
          v-model="form.timeout"
          :formatter="(value) => `${value}s`"
          :parser="(value) => value.replace('s', '')"
        />
      </a-form-model-item>

      <a-form-model-item
        prop="default_permissions"
        :label="$t('oneterm.accessControl.permissionConfig')"
        :extra="$t('oneterm.accessControl.permissionConfigTip')"
      >
        <PermissionCheckbox
          :value="form.default_permissions"
          @change="handlePermissionChange"
        />
      </a-form-model-item>

      <a-form-model-item label=" " :colon="false">
        <a-space>
          <a-button :loading="loading" @click="getConfig()">{{ $t('reset') }}</a-button>
          <a-button :loading="loading" type="primary" @click="handleSave">{{ $t('save') }}</a-button>
        </a-space>
      </a-form-model-item>
    </a-form-model>
  </div>
</template>

<script>
import { getConfig, postConfig } from '@/modules/oneterm/api/config'
import { PERMISSION_TYPE } from './constants.js'

import PermissionCheckbox from './permissionCheckbox.vue'

const DEFAULT_PERMISSIONS = Object.values(PERMISSION_TYPE).reduce((config, key) => {
  config[key] = false
  return config
}, {})

export default {
  name: 'AccessControl',
  components: {
    PermissionCheckbox
  },
  data() {
    return {
      loading: false,
      rules: {},
      form: {
        timeout: 5,
        default_permissions: { ...DEFAULT_PERMISSIONS }
      }
    }
  },
  mounted() {
    this.getConfig()
  },
  methods: {
    getConfig() {
      getConfig({
        info: true
      }).then((res) => {
        const { timeout = 5, default_permissions } = res?.data
        this.form = {
          timeout,
          default_permissions: default_permissions || { ...DEFAULT_PERMISSIONS }
        }
      })
    },
    handlePermissionChange(key, checked) {
      this.form.default_permissions[key] = checked
    },
    handleSave() {
      this.loading = true
      postConfig({ ...this.form })
        .then(() => {
          this.$message.success(this.$t('saveSuccess'))
        })
        .finally(async () => {
          this.getConfig()
          this.loading = false
        })
    },
  },
}
</script>

<style lang="less" scoped>
.access-control {
  background-color: #fff;
  height: 100%;
  padding: 18px 0px;
  border-radius: 6px;

  &-label {
    display: inline-flex;
    align-items: center;

    &-icon {
      margin-left: 6px;
      color: @text-color_3;
    }
  }
}
</style>
