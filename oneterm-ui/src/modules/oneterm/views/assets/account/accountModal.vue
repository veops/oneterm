<template>
  <a-modal :title="title" :visible="visible" @cancel="handleCancel" @ok="handleOk" :confirmLoading="loading">
    <a-form-model ref="accountForm" :model="form" :rules="rules" :label-col="{ span: 7 }" :wrapper-col="{ span: 14 }">
      <a-form-model-item :label="$t(`oneterm.name`)" prop="name">
        <a-input v-model="form.name" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <a-form-model-item
        :label="$t(`oneterm.account`)"
        prop="account"
        :label-col="{ span: 7 }"
        :wrapper-col="{ span: 14 }"
      >
        <a-input v-model="form.account" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <CredentialEditor
        v-model="form"
        :editing="!!form.id"
        :replace.sync="replaceCredential"
        :original-type="originalType"
      />
    </a-form-model>
  </a-modal>
</template>

<script>
import { postAccount, putAccountById } from '../../../api/account'
import CredentialEditor from '../../../components/credentialEditor.vue'

export default {
  name: 'AccountModal',
  components: { CredentialEditor },
  data() {
    return {
      visible: false,
      replaceCredential: false,
      originalType: 1,
      form: {
        name: '',
        account_type: 1,
        account: '',
        password: '',
        pk: '',
        phrase: '',
      },
      rules: {
        name: [{ required: true, message: this.$t('placeholder1') }],
        account: [{ required: true, message: this.$t('placeholder1') }],
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
      this.replaceCredential = false
      this.originalType = Number(data?.account_type) || 1
      this.form = {
        id: data?.id,
        name: data?.name || '',
        account: data?.account || '',
        account_type: this.originalType,
        password: '',
pk: '',
phrase: ''
      }
      this.visible = true
      this.$nextTick(() => this.$refs.accountForm.clearValidate())
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
          if (!this.form.id || this.replaceCredential) {
            if (account_type === 1) {
              params.password = password
            } else {
              params.pk = pk
              params.phrase = phrase
            }
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
