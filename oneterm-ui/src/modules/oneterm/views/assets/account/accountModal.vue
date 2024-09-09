<template>
  <a-modal :title="title" :visible="visible" @cancel="handleCancel" @ok="handleOk" :confirmLoading="loading">
    <a-form-model ref="accountForm" :model="form" :rules="rules" :label-col="{ span: 7 }" :wrapper-col="{ span: 14 }">
      <a-form-model-item :label="$t(`oneterm.name`)" prop="name">
        <a-input v-model="form.name" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.accountType`)" prop="account_type">
        <a-radio-group v-model="form.account_type">
          <a-radio :value="1">
            {{ $t('oneterm.password') }}
          </a-radio>
          <a-radio :value="2">
            {{ $t('oneterm.secretkey') }}
          </a-radio>
        </a-radio-group>
      </a-form-model-item>
      <a-form-model-item
        :label="$t(`oneterm.account`)"
        prop="account"
        :label-col="{ span: 7 }"
        :wrapper-col="{ span: 14 }"
      >
        <a-input v-model="form.account" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item
        :label="form.account_type === 1 ? $t('oneterm.password') : $t('oneterm.secretkey')"
        prop="password"
        :label-col="{ span: 7 }"
        :wrapper-col="{ span: 14 }"
      >
        <a-input v-if="form.account_type === 1" v-model="form.password" :placeholder="`${$t(`placeholder1`)}`" />
        <a-textarea v-else v-model="form.pk" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item
        :label="$t('oneterm.phrase')"
        prop="phrase"
        :label-col="{ span: 7 }"
        :wrapper-col="{ span: 14 }"
        v-if="form.account_type === 2"
      >
        <a-input v-model="form.phrase" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import { postAccount, putAccountById } from '../../../api/account'

export default {
  name: 'AccountModal',
  data() {
    return {
      visible: false,
      form: {
        name: '',
        account_type: 1,
        account: '',
        password: '',
        pk: '',
        phrase: '',
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
        return this.$t('oneterm.assetList.editAccount')
      }
      return this.$t('oneterm.assetList.createAccount')
    },
  },
  methods: {
    open(data) {
      this.visible = true
      if (data) {
        this.form = { ...data }
      }
    },
    handleCancel() {
      this.$refs.accountForm.resetFields()
      this.form = {
        name: '',
        account_type: 1,
        account: '',
        password: '',
        pk: '',
        phrase: '',
      }
      this.visible = false
    },
    async handleOk() {
      this.$refs.accountForm.validate(async (valid) => {
        if (valid) {
          this.loading = true
          const { name, account_type, account, password, pk, phrase } = this.form
          const params = { name, account_type, account }
          if (account_type === 1) {
            params.password = password
          } else {
            params.pk = pk
            params.phrase = phrase
          }
          if (this.form.id) {
            await putAccountById(this.form.id, params)
              .then(() => {
                this.$message.success(this.$t('editSuccess'))
              })
              .finally(() => {
                this.loading = false
              })
          } else {
            await postAccount(params)
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
