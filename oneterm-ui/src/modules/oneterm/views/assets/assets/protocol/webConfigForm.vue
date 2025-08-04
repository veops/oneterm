<template>
  <div class="form-wrap">
    <a-divider orientation="left">
      <div
        class="form-divider-text"
        @click="showForm = !showForm"
      >
        <span>
          {{ $t('oneterm.web.webProtocolConfig') }}
        </span>
        <a-icon
          :type="showForm ? 'caret-up' : 'caret-down'"
        />
      </div>
    </a-divider>

    <div class="form-container" v-show="showForm">
      <!-- <div class="form-title">{{ $t('oneterm.web.authConfig') }}</div>

      <a-form-model-item
        prop="web_config.auth_mode"
        :label="$t('oneterm.web.authMode')"
        :extra="$t(authModeExtra)"
        v-bind="formItemCol"
      >
        <a-radio-group v-model="config.auth_mode">
          <a-radio
            v-for="(option) in authModeOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ $t(option.label) }}
          </a-radio>
        </a-radio-group>
      </a-form-model-item>

      <a-form-model-item
        v-if="config.auth_mode === AUTH_MODE.SMART"
        prop="web_config.login_accounts"
        :label="$t('oneterm.web.autoLoginAccount')"
        v-bind="formItemCol"
      >
        <vxe-table
          ref="xTable"
          size="mini"
          :data="config.login_accounts"
          :min-height="60"
        >
          <vxe-column field="username" :title="$t('oneterm.account')">
            <template #default="{ row }">
              <a-input
                v-model="row.username"
                :placeholder="$t('placeholder1')"
              />
            </template>
          </vxe-column>

          <vxe-column field="password" :title="$t('oneterm.password')">
            <template #default="{ row }">
              <a-input-password
                v-model="row.password"
                :placeholder="$t('placeholder1')"
              />
            </template>
          </vxe-column>

          <vxe-column field="operation" :title="$t('operation')" width="55" fixed="right">
            <template #default="{ rowIndex }">
              <a-space>
                <a @click="addAccount">
                  <a-icon type="plus-circle"/>
                </a>
                <a
                  v-if="config.login_accounts.length > 1"
                  @click="deleteAccount(rowIndex)"
                >
                  <a-icon type="minus-circle"/>
                </a>
              </a-space>
            </template>
          </vxe-column>

          <div class="auto-login-account-empty" slot="empty">
            <a @click="addAccount">
              <span>{{ $t('oneterm.web.addAccount') }}</span>
              <a-icon type="plus-circle" />
            </a>
          </div>
        </vxe-table>
      </a-form-model-item> -->

      <div class="form-title">{{ $t('oneterm.web.accessPolicy') }}</div>
      <a-form-model-item
        prop="web_config.access_policy"
        :label="$t('oneterm.web.accessMode')"
        v-bind="formItemCol"
      >
        <a-radio-group v-model="config.access_policy">
          <a-radio
            v-for="(option) in accessPolicyOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ $t(option.label) }}
          </a-radio>
        </a-radio-group>
      </a-form-model-item>

      <a-form-model-item
        v-if="config.access_policy === ACCESS_POLICY.READ_ONLY"
        prop="web_config.proxy_settings.allowed_methods"
        :label="$t('oneterm.web.allowedMethods')"
        v-bind="formItemCol"
      >
        <a-select
          v-model="config.proxy_settings.allowed_methods"
          mode="multiple"
          :placeholder="$t('placeholder2')"
          :options="allowedMethodsOptions"
        />
      </a-form-model-item>

      <a-form-model-item
        prop="web_config.proxy_settings.max_concurrent"
        :label="$t('oneterm.web.maxConcurrent')"
        :extra="$t('oneterm.web.maxConcurrentTip')"
        v-bind="formItemCol"
      >
        <a-input-number
          v-model="config.proxy_settings.max_concurrent"
          :placeholder="$t('placeholder1')"
          :min="1"
          :precision="0"
        />
      </a-form-model-item>

      <a-form-model-item
        prop="web_config.proxy_settings.blocked_paths"
        :label="$t('oneterm.web.blockedPaths')"
        v-bind="formItemCol"
      >
        <a-select
          mode="tags"
          :value="config.proxy_settings.blocked_paths"
          :options="blockedPathOptions"
          :placeholder="$t('placeholder2')"
          @change="handlePathsChange"
        />
      </a-form-model-item>

      <!-- <a-form-model-item
        prop="web_config.proxy_settings.recording_enabled"
        :label="$t('oneterm.web.enableRecording')"
        v-bind="formItemCol"
      >
        <a-switch
          v-model="config.proxy_settings.recording_enabled"
        />
      </a-form-model-item> -->

      <a-form-model-item
        prop="web_config.proxy_settings.watermark_enabled"
        :label="$t('oneterm.web.enableWatermark')"
        v-bind="formItemCol"
      >
        <a-switch
          v-model="config.proxy_settings.watermark_enabled"
        />
      </a-form-model-item>
    </div>
  </div>
</template>

<script>
import _ from 'lodash'
import {
  ACCESS_POLICY_NAME,
  AUTH_MODE,
  AUTH_MODE_NAME,
  ACCESS_POLICY
} from './constants.js'

export default {
  name: 'WebConfigForm',
  props: {
    config: {
      type: Object,
      required: true
    }
  },
  data() {
    return {
      AUTH_MODE,
      ACCESS_POLICY,
      showForm: true,
      accessPolicyOptions: [ACCESS_POLICY.FULL_ACCESS, ACCESS_POLICY.READ_ONLY].map((value) => ({
        label: ACCESS_POLICY_NAME[value],
        value: value
      })),
      allowedMethodsOptions: ['GET', 'HEAD', 'OPTIONS'].map((value) => ({
        label: value,
        value: value
      })),
      authModeOptions: [AUTH_MODE.NONE, AUTH_MODE.SMART, AUTH_MODE.MANUAL].map((value) => ({
        label: AUTH_MODE_NAME[value],
        value
      })),
      blockedPathOptions: [],
      formItemCol: {
        labelCol: { span: 6 },
        wrapperCol: { span: 18 },
      }
    }
  },
  computed: {
    // authModeExtra() {
    //   switch (this.config.auth_mode) {
    //     case AUTH_MODE.NONE:
    //       return 'oneterm.web.noAuthenticationRequiredTip'
    //     case AUTH_MODE.SMART:
    //       return 'oneterm.web.autoLoginTip'
    //     case AUTH_MODE.MANUAL:
    //       return 'oneterm.web.manualLoginTip'
    //     default:
    //       return ''
    //   }
    // },
  },
  methods: {
    handlePathsChange(value) {
      this.config.proxy_settings.blocked_paths = value
      const blockedPathOptions = this.blockedPathOptions.concat(value.map((item) => ({
        label: item,
        value: item
      })))
      this.blockedPathOptions = _.uniqBy(blockedPathOptions, 'value')
    },
    addAccount() {
      this.config.login_accounts.push({
        password: '',
        username: ''
      })
    },
    deleteAccount(index) {
      this.config.login_accounts.splice(index, 1)
    }
  }
}
</script>

<style lang="less" scoped>
.form-wrap {
  padding: 20px;
  background-color: @primary-color_7;
  margin-top: 16px;
  border-radius: 2px;

  .form-container {
    margin-top: 20px;
  }

  .form-title {
    margin-bottom: 16px;
    font-weight: 600;
    color: @text-color_1;
  }

  .form-divider-text {
    font-size: 14px;
    font-weight: 400;
    cursor: pointer;
  }

  .auto-login-account-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    height: 100%;
    font-size: 14px;

    span {
      margin-right: 6px;
    }
  }

  .ant-divider {
    margin-top: 0px;
    margin-bottom: 0px;
  }

  .ant-input-number {
    width: 100%;
  }
}
</style>
