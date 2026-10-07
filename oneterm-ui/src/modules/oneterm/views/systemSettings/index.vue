<template>
  <div class="system-settings">
    <a-tabs class="system-settings-menu" tabPosition="left" :activeKey="activeKey" @change="handleChangeTab">
      <a-tab-pane v-for="item in menuList" :key="item.key">
        <div class="system-settings-menu-title" slot="tab">
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
import { mixinPermissions } from '@/utils/mixin'

import AccessControl from './accessControl/index.vue'
import PublicKey from './publicKey/index.vue'
import QuickCommand from './quickCommand/index.vue'
import TerminalDisplay from './terminalDisplay/index.vue'
import StorageConfig from './storageConfig/index.vue'

const systemSettingTabStorageKey = 'ops_oneterm_system_setting_tab_key'

export default {
  name: 'SystemSettings',
  mixins: [mixinPermissions],
  components: {
    AccessControl,
    PublicKey,
    QuickCommand,
    TerminalDisplay,
    StorageConfig,
  },
  data() {
    return {
      activeKey: localStorage.getItem(systemSettingTabStorageKey) || '',
    }
  },
  computed: {
    ...mapState({
      roles: (state) => state.user.roles,
      detailPermissions: (state) => state.user.detailPermissions,
    }),
    isAdmin() {
      const permissions = this?.roles?.permissions || []
      const isAdmin =
        permissions?.includes?.('admin') ||
        permissions?.includes?.('oneterm_admin') ||
        permissions?.includes?.('acl_admin')
      return isAdmin
    },
    menuList() {
      const menuList = [
        {
          label: 'oneterm.systemSettings.publicKey',
          icon: 'ops-oneterm-publickey',
          key: 'publicKey',
          component: 'PublicKey',
          permission: 'public_key',
        },
        {
          label: 'oneterm.systemSettings.quickCommand',
          icon: 'quick_commands',
          key: 'quickCommand',
          component: 'QuickCommand',
          permission: 'quick_command',
        },
        {
          label: 'oneterm.systemSettings.terminalDisplay',
          icon: 'terminal_settings',
          key: 'terminalDisplay',
          component: 'TerminalDisplay',
          permission: 'terminal_show',
        },
        {
          label: 'oneterm.systemSettings.accessControl',
          icon: 'basic_settings',
          key: 'accessControl',
          component: 'AccessControl',
          permission: 'terminal_control',
        },
        {
          label: 'oneterm.systemSettings.storageConfig',
          icon: 'itsm-default_line',
          key: 'storageConfig',
          component: 'StorageConfig',
          permission: 'storage_config',
        },
      ]

      return menuList.filter(
        (item) =>
          item.available !== false &&
          (this.isAdmin || (!item.adminOnly && this.hasDetailPermission('oneterm', 'System_Config', [item.permission])))
      )
    },
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
    },
  },
  methods: {
    handleChangeTab(key) {
      localStorage.setItem(systemSettingTabStorageKey, key)
      this.activeKey = key
    },
  },
}
</script>

<style lang="less" scoped>
@import '../../style/index.less';

.system-settings {
  width: 100%;
  height: 100%;

  &-menu {
    height: 100%;

    &-title {
      display: flex;
      align-items: center;
      column-gap: 10px;
      font-size: 14px;
      font-weight: 500;
      transition: all 0.2s ease;

      i {
        font-size: 16px;
      }
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
<style lang="less">
@import '../../style/index.less';

.system-settings {
  .system-settings-menu {
    /deep/ .ant-tabs-bar,
    /deep/ .ant-tabs-content {
      border-left: 0;
      border-right: 0;
    }

    /deep/ .ant-tabs-nav-wrap {
      background-color: #fafafa;
      padding: 16px 0;
    }

    /deep/ .ant-tabs-tab {
      padding: 12px 24px;
      margin: 0;
      transition: all 0.2s ease;

      &:hover {
        background-color: fade(@primary-color, 5%);
        color: @primary-color;

        i {
          color: @primary-color;
        }
      }
    }

    /deep/ .ant-tabs-tab-active {
      background-color: fade(@primary-color, 10%);
      font-weight: 600;
      color: @primary-color;

      i {
        color: @primary-color;
      }
    }

    /deep/ .ant-tabs-ink-bar {
      display: none !important;
    }

    /deep/ .ant-tabs-content {
      padding-left: 20px;
    }
  }
}
</style>
