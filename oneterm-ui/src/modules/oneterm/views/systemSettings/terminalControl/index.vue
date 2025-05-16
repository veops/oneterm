<template>
  <div class="terminal-control">
    <a-form-model ref="configForm" :model="form" :rules="rules" :label-col="{ span: 7 }" :wrapper-col="{ span: 14 }">
      <a-form-model-item :label="$t('oneterm.terminalControl.timeout')" prop="timeout">
        <a-input-number
          :min="0"
          :max="7200"
          v-model="form.timeout"
          :formatter="(value) => `${value}s`"
          :parser="(value) => value.replace('s', '')"
        />
      </a-form-model-item>

      <a-form-model-item>
        <div class="terminal-control-label" slot="label">
          <span>RDP</span>
          <a-tooltip>
            <div slot="title">
              <p>{{ $t('oneterm.terminalControl.copyTip') }}</p>
              <p>{{ $t('oneterm.terminalControl.pasteTip') }}</p>
            </div>
            <a-icon
              type="info-circle"
              class="terminal-control-label-icon"
            />
          </a-tooltip>
        </div>
        <a-checkbox v-model="form.rdp_config.copy">
          {{ $t('oneterm.terminalControl.allowCopy') }}
        </a-checkbox>
        <a-checkbox v-model="form.rdp_config.paste">
          {{ $t('oneterm.terminalControl.allowPaste') }}
        </a-checkbox>
      </a-form-model-item>

      <a-form-model-item>
        <div class="terminal-control-label" slot="label">
          <span>VNC</span>
          <a-tooltip>
            <div slot="title">
              <p>{{ $t('oneterm.terminalControl.copyTip') }}</p>
              <p>{{ $t('oneterm.terminalControl.pasteTip') }}</p>
            </div>
            <a-icon
              type="info-circle"
              class="terminal-control-label-icon"
            />
          </a-tooltip>
        </div>
        <a-checkbox v-model="form.vnc_config.copy">
          {{ $t('oneterm.terminalControl.allowCopy') }}
        </a-checkbox>
        <a-checkbox v-model="form.vnc_config.paste">
          {{ $t('oneterm.terminalControl.allowPaste') }}
        </a-checkbox>
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
export default {
  name: 'TerminalControl',
  data() {
    return {
      loading: false,
      form: {
        timeout: 5,
        rdp_config: {
          copy: false,
          paste: false,
        },
        vnc_config: {
          copy: false,
          paste: false,
        }
      },
      rules: {},
    }
  },
  mounted() {
    this.getConfig()
  },
  methods: {
    async getConfig() {
      await getConfig({
        info: true
      }).then((res) => {
        const { timeout = 5, rdp_config, vnc_config } = res?.data
        this.form = {
          timeout,
          vnc_config: {
            copy: vnc_config?.copy || false,
            paste: vnc_config?.paste || false
          },
          rdp_config: {
            copy: rdp_config?.copy || false,
            paste: rdp_config?.paste || false
          }
        }
      })
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
.terminal-control {
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
