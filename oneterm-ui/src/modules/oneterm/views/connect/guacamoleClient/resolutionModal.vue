<template>
  <a-modal
    :title="$t('oneterm.terminalDisplay.resolution')"
    :visible="visible"
    :width="400"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-form-model
      ref="resolutionFormRef"
      :model="form"
      :label-col="{ span: 7 }"
      :wrapper-col="{ span: 14 }"
    >
      <a-form-model-item :label="$t('oneterm.terminalDisplay.resolution')" prop="resolution">
        <ResolutionSetting
          v-model="form.resolution"
        />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import { putPreference } from '@/modules/oneterm/api/preference.js'

import ResolutionSetting from '@/modules/oneterm/views/systemSettings/terminalDisplay/resolutionSetting/index.vue'

export default {
  name: 'ResolutionModal',
  components: {
    ResolutionSetting
  },
  data() {
    return {
      visible: false,
      form: {
        resolution: 'auto'
      },
    }
  },
  methods: {
    open(resolution) {
      this.visible = true
      this.form = {
        resolution,
      }
    },
    handleCancel() {
      this.visible = false
      this.form = {
        resolution: 'auto',
      }
      this.$refs.resolutionFormRef.resetFields()
    },
    handleOk() {
      this.$refs.resolutionFormRef.validate((valid) => {
        if (valid) {
          putPreference({
            settings: {
              resolution: this.form.resolution
            }
          }).then(() => {
            this.$emit('ok')
            this.handleCancel()
          })
        }
      })
    },
  },
}
</script>
