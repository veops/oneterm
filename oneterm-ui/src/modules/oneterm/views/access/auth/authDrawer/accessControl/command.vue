<template>
  <a-form-model-item :label="$t('oneterm.auth.commandFilter')" prop="access_control.cmd_ids">
    <a-select
      mode="multiple"
      :value="form.access_control.cmd_ids"
      :options="commandSelectOptions"
      :placeholder="$t('oneterm.auth.commandTip1')"
      @change="(value) => $emit('change', ['access_control', 'cmd_ids'], value)"
    />
    <a-select
      mode="multiple"
      :value="form.access_control.template_ids"
      :options="commandTemplateSelectOptions"
      :placeholder="$t('oneterm.auth.commandTip2')"
      @change="(value) => $emit('change', ['access_control', 'template_ids'], value)"
    />
  </a-form-model-item>
</template>

<script>
import { getCommandList } from '@/modules/oneterm/api/command.js'
import { getCommandTemplateList } from '@/modules/oneterm/api/commandTemplate.js'

export default {
  name: 'Command',
  props: {
    form: {
      type: Object,
      default: () => {}
    }
  },
  data() {
    return {
      commandSelectOptions: [],
      commandTemplateSelectOptions: [],
    }
  },
  mounted() {
    this.getCommandList()
    this.getCommandTemplateList()
  },
  methods: {
    getCommandList() {
      getCommandList({
        page_index: 1,
        page_size: 9999
      }).then((res) => {
        const list = res?.data?.list || []
        this.commandSelectOptions = list.map((item) => ({
          value: item.id,
          label: item.name
        }))
      })
    },
    getCommandTemplateList() {
      getCommandTemplateList({
        page_index: 1,
        page_size: 9999
      }).then((res) => {
        const list = res?.data?.list || []
        this.commandTemplateSelectOptions = list.map((item) => ({
          value: item.id,
          label: item.name
        }))
      })
    },
  }
}
</script>
