<template>
  <a-tabs
    type="card"
    class="ops-tab command-tab"
  >
    <a-tab-pane
      key="commandManagement"
      :tab="$t('oneterm.commandFilter.commandManagement')"
      :forceRender="true"
    >
      <CommandManagement />
    </a-tab-pane>
    <a-tab-pane
      key="commandTemplateManagement"
      :tab="$t('oneterm.commandFilter.commandTemplateManagement')"
      :forceRender="true"
    >
      <CommandTemplateManagement />
    </a-tab-pane>
  </a-tabs>
</template>

<script>
import CommandManagement from './commandManagement/index.vue'
import CommandTemplateManagement from './commandTemplateManagement/index.vue'

export default {
  name: 'AccessCommand',
  components: {
    CommandManagement,
    CommandTemplateManagement
  }
}
</script>

<style lang="less" scoped>
.command-tab {
  height: 100%;
}
</style>
