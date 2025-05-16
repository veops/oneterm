<template>
  <a-modal
    :title="title"
    :visible="visible"
    :confirmLoading="confirmLoading"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-form-model
      ref="commandModalForm"
      :model="form"
      :rules="rules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 17 }"
    >
      <a-form-model-item :label="`${$t('name')}`" prop="name">
        <a-input v-model="form.name" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <a-form-model-item :label="`${$t('description')}`" prop="description">
        <a-input v-model="form.description" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.command`)" prop="command">
        <a-textarea
          v-model="form.command"
          :placeholder="$t('placeholder1')"
          :rows="4"
        />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import { postQuickCommand, putQuickCommandById } from '@/modules/oneterm/api/quickCommand.js'

export default {
  name: 'CommandModal',
  data() {
    return {
      visible: false,
      confirmLoading: false,

      commandId: '',
      form: {
        name: '',
        command: '',
        description: ''
      },
      rules: {
        name: [{ required: true, message: this.$t('placeholder1') }],
        command: [{ required: true, message: this.$t('placeholder1') }],
      }
    }
  },
  computed: {
    title() {
      if (this.commandId) {
        return this.$t('oneterm.quickCommand.editCommand')
      }
      return this.$t('oneterm.quickCommand.createCommand')
    },
  },
  methods: {
    open(data) {
      this.visible = true
      if (data) {
        this.form = {
          name: data?.name || '',
          command: data?.command || '',
          description: data?.description || ''
        }
        this.commandId = data?.id || ''
      }
    },

    handleCancel() {
      this.visible = false
      this.form = {
        name: '',
        command: '',
        description: ''
      }
      this.commandId = ''
      this.confirmLoading = false
      this.$refs.commandModalForm.resetFields()
    },

    handleOk() {
      this.$refs.commandModalForm.validate(async (valid) => {
        if (valid) {
          this.confirmLoading = true
          if (this.commandId) {
            await putQuickCommandById(
              this.commandId,
              this.form
            )
          } else {
            await postQuickCommand(this.form)
          }

          this.handleCancel()
          this.$emit('ok')
        }
      })
    }
  }
}
</script>

<style lang="less" scoped>
</style>
