<template>
  <a-tabs type="card" class="ops-tab">
    <a-tab-pane key="1" :tab="$t('oneterm.assetList.commandFilter')">
      <Command />
    </a-tab-pane>
    <a-tab-pane v-if="isAdmin" key="2" :tab="$t('oneterm.assetList.basicSettings')">
      <BasicSetting />
    </a-tab-pane>
  </a-tabs>
</template>

<script>
import { mapState } from 'vuex'
import Command from './command.vue'
import BasicSetting from './basicSetting.vue'

export default {
  name: 'Security',
  components: { Command, BasicSetting },
  computed: {
    ...mapState({
      roles: (state) => state.user.roles,
    }),
    isAdmin() {
      const permissions = this?.roles?.permissions || []
      const isAdmin = permissions?.includes?.('oneterm_admin') || permissions?.includes?.('acl_admin')
      return isAdmin
    }
  }
}
</script>

<style></style>
