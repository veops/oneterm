<template>
  <a-modal
    :title="$t('oneterm.terminalDisplay.custom')"
    :visible="visible"
    :width="400"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-form-model
      ref="customFormRef"
      :model="form"
      :rules="rules"
      :label-col="{ span: 7 }"
      :wrapper-col="{ span: 14 }"
    >
      <a-form-model-item :label="$t('oneterm.terminalDisplay.width')" prop="width">
        <a-input-number
          v-model="form.width"
          :placeholder="$t('placeholder1')"
          :max="10000"
          :min="800"
          :step="1"
          :precision="0"
          style="width: 200px"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.terminalDisplay.height')" prop="height">
        <a-input-number
          v-model="form.height"
          :placeholder="$t('placeholder1')"
          :max="10000"
          :min="600"
          :step="1"
          :precision="0"
          style="width: 200px"
        />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
export default {
  name: 'CustomModal',
  data() {
    return {
      visible: false,
      form: {
        width: 800,
        height: 600
      },
      rules: {
        width: [{ required: true, message: this.$t('placeholder1') }],
        height: [{ required: true, message: this.$t('placeholder1') }],
      }
    }
  },
  methods: {
    open(value) {
      this.visible = true
      const [width, height] = value.split('x')
      this.form = {
        width,
        height
      }
    },
    handleCancel() {
      this.visible = false
      this.form = {
        width: 800,
        height: 600
      }
      this.$refs.customFormRef.resetFields()
    },
    handleOk() {
      this.$refs.customFormRef.validate(async (valid) => {
        if (valid) {
          this.$emit('ok', `${this.form.width}x${this.form.height}`)
          this.handleCancel()
        }
      })
    },
  },
}
</script>
