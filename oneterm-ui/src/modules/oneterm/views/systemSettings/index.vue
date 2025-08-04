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
import { mapState } from 'vuex'

import AccessControl from './accessControl/index.vue'
import PublicKey from './publicKey/index.vue'
import QuickCommand from './quickCommand/index.vue'
import TerminalDisplay from './terminalDisplay/index.vue'
import StorageConfig from './storageConfig/index.vue'

const systemSettingTabStorageKey = 'ops_oneterm_system_setting_tab_key'

export default {
  name: 'SystemSettings',
  components: {
    AccessControl,
    PublicKey,
    QuickCommand,
    TerminalDisplay,
    StorageConfig
  },
  data() {
    return {
      activeKey: localStorage.getItem(systemSettingTabStorageKey) || '',
    }
  },
  computed: {
    ...mapState({
      roles: (state) => state.user.roles
    }),
    isAdmin() {
      const permissions = this?.roles?.permissions || []
      const isAdmin = permissions?.includes?.('oneterm_admin') || permissions?.includes?.('acl_admin')
      return isAdmin
    },
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
        }
      ]

      if (this.isAdmin) {
        menuList.push(
          {
            label: 'oneterm.systemSettings.accessControl',
            icon: 'basic_settings',
            key: 'accessControl',
            component: 'AccessControl',
          },
          {
            label: 'oneterm.systemSettings.storageConfig',
            icon: 'itsm-default_line',
            key: 'storageConfig',
            component: 'StorageConfig'
          }
        )
      }

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
