<template>
  <div class="system-settings">
    <a-tabs
      class="system-settings-menu"
      tabPosition="left"
      :activeKey="activeKey"
      @change="handleChangeTab"
    >
      <a-tab-pane
        v-for="(item) in menuList"
        :key="item.key"
      >
        <div
          class="system-settings-menu-title"
          slot="tab"
        >
          <ops-icon :type="item.icon" />
          <span>{{ $t(item.label) }}</span>
        </div>
        <components :is="item.component" />
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script>
import { mixinPermissions } from '@/utils/mixin'

import CommandIntercept from './commandIntercept/index.vue'
import TerminalControl from './terminalControl/index.vue'
import PublicKey from './publicKey/index.vue'
import QuickCommand from './quickCommand/index.vue'
import TerminalDisplay from './terminalDisplay/index.vue'

const systemSettingTabStorageKey = 'ops_oneterm_system_setting_tab_key'

export default {
  name: 'SystemSettings',
  mixins: [mixinPermissions],
  components: {
    CommandIntercept,
    TerminalControl,
    PublicKey,
    QuickCommand,
    TerminalDisplay
  },
  data() {
    return {
      activeKey: localStorage.getItem(systemSettingTabStorageKey) || '',
    }
  },
  computed: {
    menuList() {
      const menuList = [
        {
          label: 'oneterm.systemSettings.publicKey',
          icon: 'ops-oneterm-publickey',
          key: 'publicKey',
          component: 'PublicKey',
        },
        {
          label: 'oneterm.systemSettings.quickCommand',
          icon: 'quick_commands',
          key: 'quickCommand',
          component: 'QuickCommand',
        },
        {
          label: 'oneterm.systemSettings.terminalDisplay',
          icon: 'terminal_settings',
          key: 'terminalDisplay',
          component: 'TerminalDisplay'
        },
        {
          label: 'oneterm.systemSettings.terminalControl',
          icon: 'basic_settings',
          key: 'terminalControl',
          component: 'TerminalControl',
        },
        {
          label: 'oneterm.systemSettings.commandIntercept',
          icon: 'a-command_interception1',
          key: 'commandIntercept',
          component: 'CommandIntercept',
        }
      ]

      return menuList
    }
  },
  watch: {
    menuList: {
      deep: true,
      immediate: true,
      handler(menuList) {
        if (!menuList?.length) {
          return
        }

        if (!menuList.find((item) => item.key === this.activeKey)) {
          this.handleChangeTab(menuList[0].key)
        }
      },
    }
  },
  methods: {
    handleChangeTab(key) {
      localStorage.setItem(systemSettingTabStorageKey, key)
      this.activeKey = key
    }
  }
}
</script>

<style lang="less" scoped>
.system-settings {
  width: 100%;
  height: 100%;

  &-menu {
    height: 100%;

    &-title {
      display: flex;
      align-items: center;
    }
  }

  /deep/ .ant-tabs-content {
    height: 100%;
  }

  /deep/ .ant-tabs-tabpane-active {
    height: 100%;
  }
}
</style>
