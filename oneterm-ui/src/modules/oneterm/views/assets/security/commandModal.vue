<template>
  <a-modal :title="title" :visible="visible" @cancel="handleCancel" @ok="handleOk" :confirmLoading="loading">
    <a-form-model ref="commandForm" :model="form" :rules="rules" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
      <a-form-model-item :label="$t(`oneterm.name`)" prop="name">
        <a-input v-model="form.name" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.command`)" prop="cmds">
        <a-input v-model="form.cmd" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.assetList.enable`)" prop="enable">
        <a-switch v-model="form.enable" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.assetList.regexp`)" prop="enable">
        <a-switch v-model="form.is_re" />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import { postCommand, putCommandById } from '../../../api/command'

export default {
  name: 'CommandModal',
  data() {
    return {
      visible: false,
      form: {
        name: '',
        cmd: [],
        enable: true,
        is_re: true,
      },
      rules: {
        name: [{ required: true, message: `${this.$t(`placeholder1`)}` }],
      },
      loading: false,
    }
  },
  computed: {
    title() {
      if (this.form.id) {
        return this.$t('oneterm.assetList.editCommand')
      }
      return this.$t('oneterm.assetList.createCommand')
    },
  },
  methods: {
    open(data) {
      this.visible = true
      if (data) {
        this.form = {
          ...data,
          enable: Boolean(data.enable),
          is_re: Boolean(data.is_re),
          cmd: data.cmd ?? ''
        }
      }
    },
    handleCancel() {
      this.$refs.commandForm.resetFields()
      this.form = {
        name: '',
        cmd: '',
        enable: true,
        is_re: true,
      }
      this.visible = false
    },
    async handleOk() {
      this.$refs.commandForm.validate(async (valid) => {
        if (valid) {
          this.loading = true
          if (this.form.id) {
            await putCommandById(this.form.id, { ...this.form })
              .then(() => {
                this.$message.success(this.$t('editSuccess'))
              })
              .finally(() => {
                this.loading = false
              })
          } else {
            await postCommand({ ...this.form })
              .then(() => {
                this.$message.success(this.$t('createSuccess'))
              })
              .finally(() => {
                this.loading = false
              })
          }
          this.$emit('submit')
          this.handleCancel()
        }
      })
    },
  },
}
</script>

<style></style>
