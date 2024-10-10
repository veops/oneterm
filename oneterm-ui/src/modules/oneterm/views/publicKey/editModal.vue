<template>
  <a-modal :title="title" :visible="visible" @cancel="handleCancel" @ok="handleOK" :confirmLoading="loading">
    <a-form-model ref="editForm" :model="form" :rules="rules" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
      <a-form-model-item prop="name" :label="$t(`oneterm.name`)">
        <a-input v-model="form.name" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item prop="pk" :label="$t(`oneterm.publicKey`)">
        <a-textarea v-model="form.pk" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <!-- <a-form-model-item prop="mac" :label="$t(`oneterm.macAddress`)">
        <a-input v-model="form.mac" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item> -->
    </a-form-model>
  </a-modal>
</template>
<script>
import { addPublicKey, putPublicKeyById } from '../../api/publicKey'
export default {
  name: 'EditModal',
  data() {
    return {
      visible: false,
      type: '',
      form: {},
      loading: false,
      rules: {
        name: [{ required: true, message: `${this.$t(`placeholder1`)}`, trigger: 'blur' }],
        pk: [{ required: true, message: `${this.$t(`placeholder1`)}`, trigger: 'blur' }],
        // mac: [
        //   { required: true, message: `${this.$t(`placeholder1`)}`, trigger: 'blur' },
        // ],
      },
    }
  },
  computed: {
    title() {
      if (this.type === 'add') {
        return this.$t('oneterm.createPublicKey')
      }
      if (this.type === 'edit') {
        return this.$t('oneterm.editPublicKey')
      }
      return ''
    },
  },
  methods: {
    open(type, row) {
      this.type = type
      this.visible = true
      if (type === 'add') {
        this.form = {
          name: '',
          // mac: '',
          pk: '',
        }
      }
      if (this.type === 'edit') {
        this.form = { ...row }
      }
    },
    handleCancel() {
      this.$refs.editForm.resetFields()
      this.form = {
        name: '',
        // mac: '',
        pk: '',
      }
      this.visible = false
    },
    handleOK() {
      this.$refs.editForm.validate(async (valid) => {
        if (valid) {
          this.loading = true
          if (this.type === 'add') {
            await addPublicKey({ ...this.form })
              .then(() => {
                this.$message.success(this.$t('createSuccess'))
              })
              .finally(() => {
                this.loading = false
              })
          }
          if (this.type === 'edit') {
            await putPublicKeyById(this.form.id, { ...this.form })
              .then(() => {
                this.$message.success(this.$t('editSuccess'))
              })
              .finally(() => {
                this.loading = false
              })
          }
          this.$emit('refresh')
          this.handleCancel()
        }
      })
    },
  },
}
</script>

<style lang="less" scoped></style>
