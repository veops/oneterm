<template>
  <a-tabs
    type="card"
    class="ops-tab terminal-settings-tab"
  >
    <a-tab-pane
      key="displaySetting"
      :tab="$t('oneterm.terminalDisplay.displaySetting')"
      :forceRender="true"
    >
      <DisplaySetting />
    </a-tab-pane>
    <a-tab-pane
      key="themeSetting"
      :tab="$t('oneterm.terminalDisplay.themeSetting')"
      :forceRender="true"
    >
      <ThemeSetting />
    </a-tab-pane>
  </a-tabs>
</template>

<script>
import DisplaySetting from './displaySetting.vue'
import ThemeSetting from './themeSetting.vue'

export default {
  name: 'TerminalDisplay',
  components: {
    DisplaySetting,
    ThemeSetting
  }
}
</script>

<style lang="less" scoped>
.terminal-settings-tab {
  height: 100%;
}
</style>
