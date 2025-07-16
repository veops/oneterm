<template>
  <a-modal
    :title="title"
    :visible="visible"
    :confirmLoading="confirmLoading"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-form-model
      ref="commandFormRef"
      :model="form"
      :rules="rules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 16 }"
    >
      <a-form-model-item :label="$t('name')" prop="name">
        <a-input v-model="form.name" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <a-form-model-item :label="$t('description')" prop="description">
        <a-input v-model="form.description" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.command')" prop="cmd">
        <a-input v-model="form.cmd" :placeholder="$t('placeholder1')" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.commandFilter.riskLevel')" prop="risk_level">
        <a-select
          v-model="form.risk_level"
          :placeholder="$t('placeholder2')"
          :options="rishLevelSelectOptions"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.commandFilter.category')" prop="category">
        <a-select
          v-model="form.category"
          :placeholder="$t('placeholder2')"
          :options="categorySelectOptions"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.isEnable')" prop="enable">
        <a-switch v-model="form.enable" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.commandFilter.regexp')" prop="is_re">
        <a-switch v-model="form.is_re" />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import { COMMAND_CATEGORY, COMMAND_CATEGORY_NAME, COMMAND_RISK_NAME } from '../constants.js'
import { postCommand, putCommandById } from '@/modules/oneterm/api/command'

const DEFAULT_FORM = {
  name: '',
  description: '',
  cmd: '',
  risk_level: 0,
  category: COMMAND_CATEGORY.SECURITY,
  enable: true,
  is_re: true
}

export default {
  name: 'CommandModal',
  data() {
    return {
      visible: false,
      commandId: '',
      form: { ...DEFAULT_FORM },
      rules: {
        name: [{ required: true, message: this.$t('placeholder1') }],
      },
      confirmLoading: false
    }
  },
  computed: {
    title() {
      if (this.commandId) {
        return this.$t('oneterm.commandFilter.editCommand')
      }
      return this.$t('oneterm.commandFilter.createCommand')
    },
    categorySelectOptions() {
      return Object.values(COMMAND_CATEGORY).map((value) => {
        return {
          value,
          label: this.$t(COMMAND_CATEGORY_NAME[value])
        }
      })
    },
    rishLevelSelectOptions() {
      return [0, 1, 2, 3].map((value) => {
        return {
          value,
          label: this.$t(COMMAND_RISK_NAME[value])
        }
      })
    }
  },
  methods: {
    open(data) {
      this.visible = true
      if (data) {
        this.form = {
          name: data?.name ?? '',
          description: data?.description ?? '',
          cmd: data.cmd ?? '',
          category: data?.category ?? COMMAND_CATEGORY.SECURITY,
          risk_level: data?.risk_level ?? 0,
          enable: Boolean(data.enable),
          is_re: Boolean(data.is_re)
        }
        this.commandId = data?.id ?? ''
      }
    },
    handleCancel() {
      this.form = { ...DEFAULT_FORM }
      this.commandId = ''

      this.$refs.commandFormRef.resetFields()
      this.visible = false
    },
    async handleOk() {
      this.$refs.commandFormRef.validate(async (valid) => {
        if (!valid) return
        this.confirmLoading = true
        try {
          if (this.commandId) {
            await putCommandById(this.commandId, { ...this.form })
            this.$message.success(this.$t('editSuccess'))
          } else {
            await postCommand({ ...this.form })
            this.$message.success(this.$t('createSuccess'))
          }

          this.$emit('submit')
          this.handleCancel()
        } catch (e) {
          console.error('submit error:', e)
        } finally {
          this.confirmLoading = false
        }
      })
    },
  },
}
</script>
