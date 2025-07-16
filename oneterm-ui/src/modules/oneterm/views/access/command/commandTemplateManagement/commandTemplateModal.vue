<template>
  <a-modal
    :title="title"
    :visible="visible"
    :confirmLoading="confirmLoading"
    :width="800"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <a-form-model
      ref="commandTemplateForm"
      :model="form"
      :rules="rules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 16 }"
    >
      <a-form-model-item :label="$t('name')" prop="name">
        <a-input v-model="form.name" :placeholder="$t(`placeholder1`)"/>
      </a-form-model-item>
      <a-form-model-item :label="$t('description')" prop="description">
        <a-input v-model="form.description" :placeholder="$t('placeholder1')"/>
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.commandFilter.category')" prop="category">
        <a-select
          v-model="form.category"
          :placeholder="$t('placeholder2')"
          :options="categorySelectOptions"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.commandFilter.selectCommand')" prop="cmd_ids">
        <a-transfer
          :data-source="allCommand"
          :target-keys="form.cmd_ids"
          :selected-keys="transferSelectedKeys"
          :render="item => item.title"
          :titles="[$t('oneterm.commandFilter.unselectCommand'), $t('oneterm.commandFilter.selectedCommand')]"
          :listStyle="{
            width: 'calc((100% - 40px) / 2)',
            height: '300px'
          }"
          @change="handleTransferChange"
          @selectChange="handleTransferSelectChange"
        />
      </a-form-model-item>
    </a-form-model>
  </a-modal>
</template>

<script>
import { COMMAND_CATEGORY, COMMAND_CATEGORY_NAME } from '../constants.js'
import { postCommandTemplate, putCommandTemplateById } from '@/modules/oneterm/api/commandTemplate.js'
import { getCommandList } from '@/modules/oneterm/api/command.js'

const DEFAULT_FORM = {
  name: '',
  description: '',
  category: COMMAND_CATEGORY.SECURITY,
  cmd_ids: [],
}

export default {
  name: 'CommandTemplateModal',
  data() {
    return {
      visible: false,
      commandTemplateId: '',
      form: { ...DEFAULT_FORM },
      rules: {
        name: [{ required: true, message: this.$t(`placeholder1`) }],
        cmd_ids: [{ required: true, message: this.$t(`placeholder2`) }],
      },
      allCommand: [],
      transferSelectedKeys: [],
      confirmLoading: false
    }
  },
  computed: {
    title() {
      if (this.commandTemplateId) {
        return this.$t('oneterm.commandFilter.editCommandTemplate')
      }
      return this.$t('oneterm.commandFilter.createCommandTemplate')
    },
    categorySelectOptions() {
      return Object.values(COMMAND_CATEGORY).map((value) => {
        return {
          value,
          label: this.$t(COMMAND_CATEGORY_NAME[value])
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
          category: data?.category ?? COMMAND_CATEGORY.SECURITY,
          cmd_ids: (data?.cmd_ids ?? []).map((id) => String(id))
        }
        this.commandTemplateId = data?.id ?? ''
      }
      this.getAllCommand()
    },
    async getAllCommand() {
      const res = await getCommandList({
        page_index: 1,
        page_size: 9999,
      })
      const allCommand = res?.data?.list || []
      this.allCommand = allCommand.map((item) => {
        return {
          key: String(item.id),
          title: item.name
        }
      })
      this.form.cmd_ids = this.form.cmd_ids.filter((id) => this.allCommand.some((command) => command?.key === id))
    },
    handleCancel() {
      this.form = { ...DEFAULT_FORM }
      this.allCommand = []
      this.transferSelectedKeys = []
      this.commandTemplateId = ''

      this.$refs.commandTemplateForm.resetFields()
      this.visible = false
    },
    handleTransferChange(nextTargetKeys) {
      this.form.cmd_ids = nextTargetKeys
    },
    handleTransferSelectChange(sourceSelectedKeys, targetSelectedKeys) {
      this.transferSelectedKeys = [...sourceSelectedKeys, ...targetSelectedKeys]
    },
    async handleOk() {
      this.$refs.commandTemplateForm.validate(async (valid) => {
        if (!valid) return
        this.confirmLoading = true
        try {
          const params = {
            ...this.form,
            cmd_ids: this.form.cmd_ids.map((id) => Number(id))
          }
          if (this.commandTemplateId) {
            await putCommandTemplateById(this.commandTemplateId, params)
            this.$message.success(this.$t('editSuccess'))
          } else {
            await postCommandTemplate(params)
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
