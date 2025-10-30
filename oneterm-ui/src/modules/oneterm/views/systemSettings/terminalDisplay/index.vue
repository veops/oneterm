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

  /deep/ .ant-tabs-card-bar {
    border-bottom: 1px solid #e8eaed;

    .ant-tabs-tab {
      background-color: #fafafa;
      border: 1px solid #e8eaed;
      color: @text-color_2;

      &:hover {
        color: @primary-color;
      }
    }

    .ant-tabs-tab-active {
      background-color: #fff;
      color: @primary-color;
      font-weight: 600;
    }
  }
}
</style>
