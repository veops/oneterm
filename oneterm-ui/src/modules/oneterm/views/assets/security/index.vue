<template>
  <a-tabs type="card" class="ops-tab">
    <a-tab-pane key="1" :tab="$t('oneterm.assetList.commandFilter')">
      <Command />
    </a-tab-pane>
    <a-tab-pane key="2" :tab="$t('oneterm.assetList.basicSettings')">
      <BasicSetting />
    </a-tab-pane>
  </a-tabs>
</template>

<script>
import Command from './command.vue'
import BasicSetting from './basicSetting.vue'
export default {
  name: 'Security',
  components: { Command, BasicSetting },
}
</script>

<style></style>
