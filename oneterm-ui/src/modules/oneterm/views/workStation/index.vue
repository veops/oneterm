<template>
  <div class="oneterm-workstation oneterm-layout">
    <AssetList
      :userStat="userStat"
      :selectedKeys="selectedKeys"
      @updateSelectedKeys="updateSelectedKeys"
    >
      <div
        :class="[
          'oneterm-workstation-two',
          openFullScreen ? 'oneterm-workstation-two_full' : ''
        ]"
        slot="two"
      >
        <a-tabs
          id="workstation-drag-tab"
          v-model="tabActiveKey"
        >
          <a-tab-pane :key="WORKSTATION_TAB_TYPE.MY_ASSETS">
            <template #tab>
              <div>
                <ops-icon type="veops-resource2" />
                <strong>{{ $t('oneterm.workStation.myAsset') }}</strong>
              </div>
            </template>

            <AssetTable
              v-if="loading"
              :selectedKeys="selectedKeys"
              :accountList="accountList"
              @openTerminal="openTerminal"
              @openTerminalList="openTerminalList"
            />
          </a-tab-pane>

          <template v-if="terminalList.length" >
            <a-tab-pane
              v-for="(item, index) in terminalList"
              :key="item.id"
            >
              <template #tab>
                <div class="oneterm-workstation-tab-terminal">
                  <template v-if="![WORKSTATION_TAB_TYPE.DISPLAY_SETTING, WORKSTATION_TAB_TYPE.THEME_SETTING, WORKSTATION_TAB_TYPE.BATCH_EXECUTION].includes(item.type)">
                    <a-icon
                      v-if="item.socketStatus === SOCKET_STATUS.LOADING"
                      type="loading"
                    />
                    <span
                      v-else
                      :class="['oneterm-workstation-tab-terminal-status', item.socketStatus === SOCKET_STATUS.ERROR ? 'oneterm-workstation-tab-terminal-status_error' : '']"
                    ></span>
                  </template>

                  <a-tooltip :title="item.name">
                    <span class="oneterm-workstation-tab-terminal-title">{{ item.name }}</span>
                  </a-tooltip>

                  <a-icon
                    class="oneterm-workstation-tab-terminal-icon"
                    type="close"
                    @click.stop="closeTerminal(item, index)"
                  />
                  <ops-icon
                    v-if="item.type === WORKSTATION_TAB_TYPE.TERMINAL"
                    class="oneterm-workstation-tab-terminal-icon"
                    type="veops-copy"
                    @click.stop="copyTerminal(item)"
                  />
                </div>
              </template>

              <DisplaySetting
                v-if="item.type === WORKSTATION_TAB_TYPE.DISPLAY_SETTING"
                class="oneterm-workstation-panel"
                @ok="getPreference"
              />

              <ThemeSetting
                v-else-if="item.type === WORKSTATION_TAB_TYPE.THEME_SETTING"
                class="oneterm-workstation-panel"
                @ok="getPreference"
              />

              <BatchExecution
                v-else-if="item.type === WORKSTATION_TAB_TYPE.BATCH_EXECUTION"
                :preferenceSetting="preferenceSetting"
                :batchExecutionData="item.batchExecutionData"
                @getOfUserStat="getOfUserStat()"
              />

              <TerminalPanel
                v-else-if="item.type === WORKSTATION_TAB_TYPE.TERMINAL"
                class="oneterm-workstation-panel"
                :ref="'workStationPanelRef' + item.id"
                :assetId="item.assetId"
                :accountId="item.accountId"
                :protocol="item.protocol"
                :assetPermissions="item.permissions"
                :isFullScreen="false"
                :preferenceSetting="preferenceSetting"
                @close="handleTerminalSocketStatus(item, SOCKET_STATUS.ERROR)"
                @open="handleTerminalSocketStatus(item, SOCKET_STATUS.SUCCESS)"
                @openSystemSetting="openSystemSetting"
              />

              <GuacamolePanel
                v-else-if="item.type === WORKSTATION_TAB_TYPE.GUACAMOLE"
                class="oneterm-workstation-panel"
                :ref="'workStationPanelRef' + item.id"
                :assetId="item.assetId"
                :accountId="item.accountId"
                :protocol="item.protocol"
                :assetPermissions="item.permissions"
                :isFullScreen="false"
                :preferenceSetting="preferenceSetting"
                @close="handleTerminalSocketStatus(item, SOCKET_STATUS.ERROR)"
                @open="handleTerminalSocketStatus(item, SOCKET_STATUS.SUCCESS)"
                @updatePreferenceSetting="getPreference"
              />
            </a-tab-pane>
          </template>

          <a-icon
            slot="tabBarExtraContent"
            :type="showOperationMenu ? 'menu-unfold' : 'menu-fold'"
            class="operation-menu-icon"
            @click="toggleOperationMenu"
          />
        </a-tabs>

        <OperationMenu
          :style="{
            width: showOperationMenu ? '40px' : '0px'
          }"
          :openFullScreen="openFullScreen"
          :accountList="accountList"
          :currentTabData="currentTabData"
          @toggleFullScreen="toggleFullScreen"
          @openRecentSession="openRecentSession"
          @openBatchExecution="openBatchExecution"
          @openSystemSetting="openSystemSetting"
          @callComponentFn="callComponentFn"
        />
      </div>
    </AssetList>

    <RecentSession
      ref="recentSessionRef"
      @openTerminal="openTerminal"
    />
  </div>
</template>

<script>
import _ from 'lodash'
import { v4 as uuidv4 } from 'uuid'
import { mapState } from 'vuex'
import Sortable from 'sortablejs'
import { getOfUserStat } from '@/modules/oneterm/api/stat'
import { getPreference } from '@/modules/oneterm/api/preference.js'
import { getAccountList } from '@/modules/oneterm/api/account'
import { getConfig } from '@/modules/oneterm/api/config'
import { getAssetPermissions } from '@/modules/oneterm/api/asset'
import { defaultPreferenceSetting } from '../systemSettings/terminalDisplay/constants.js'
import { WORKSTATION_TAB_TYPE, SOCKET_STATUS } from './constants.js'
import FullScreenMixin from '@/modules/oneterm/mixins/fullScreenMixin'

import RecentSession from './recentSession.vue'
import AssetList from './asset/assetList.vue'
import TerminalPanel from '@/modules/oneterm/views/connect/terminal/index.vue'
import GuacamolePanel from '@/modules/oneterm/views/connect/guacamoleClient/index.vue'
import DisplaySetting from '../systemSettings/terminalDisplay/displaySetting.vue'
import ThemeSetting from '../systemSettings/terminalDisplay/themeSetting.vue'
import AssetTable from './asset/assetTable.vue'
import BatchExecution from './batchExecution/index.vue'
import OperationMenu from './operationMenu/index.vue'

const operationMenuExpandKey = 'ops_oneterm_work_station_menu_expand'

export default {
  name: 'WorkStation',
  mixins: [FullScreenMixin],
  components: {
    RecentSession,
    AssetList,
    TerminalPanel,
    GuacamolePanel,
    DisplaySetting,
    ThemeSetting,
    AssetTable,
    BatchExecution,
    OperationMenu
  },
  data() {
    return {
      userStat: {},
      terminalList: [],
      tabActiveKey: WORKSTATION_TAB_TYPE.MY_ASSETS,
      preferenceSetting: {
        ...defaultPreferenceSetting,
      },
      sortableInstance: null,
      selectedKeys: [],
      accountList: [],
      loading: false,
      WORKSTATION_TAB_TYPE,
      SOCKET_STATUS,
      showOperationMenu: localStorage.getItem(operationMenuExpandKey) ? localStorage.getItem(operationMenuExpandKey) === 'true' : true,
      controlConfig: {},
    }
  },
  computed: {
    ...mapState({
      personName: (state) => state.user.name,
      personRoles: (state) => state.user.roles,
      personAvatar: (state) => state.user.avatar,
      last_login: (state) => state.user.last_login,
    }),
    isOnetermAdmin() {
      const permissions = this?.personRoles?.permissions || []
      const isAdmin = permissions?.includes?.('oneterm_admin') || permissions?.includes?.('acl_admin')
      return isAdmin
    },
    currentTabData() {
      if (this.tabActiveKey === WORKSTATION_TAB_TYPE.MY_ASSETS) {
        return {
          id: WORKSTATION_TAB_TYPE.MY_ASSETS
        }
      }

      const _find = this.terminalList.find((item) => item.id === this.tabActiveKey)
      return _find
    }
  },
  mounted() {
    Promise.all([
      this.getAccountList(),
      this.getOfUserStat(),
      this.getPreference(),
      this.getConfig()
    ]).finally(() => {
      this.$nextTick(() => {
        this.initSortable()
      })
      this.loading = true
    })
  },
  beforeDestroy() {
    if (this.sortableInstance) {
      this.sortableInstance.destroy()
      this.sortableInstance = null
    }
  },
  methods: {
    async getAccountList() {
      const res = await getAccountList({ page_index: 1, info: this.forMyAsset })
      this.accountList = res?.data?.list || []
    },

    getOfUserStat: _.debounce(async function() {
      const res = await getOfUserStat({
        info: true
      })
      this.userStat = res?.data ?? {}
    }, 2000),

    async getPreference() {
      const res = await getPreference()
      const data = res?.data || {}

      const preferenceSetting = {}
      Object.keys(defaultPreferenceSetting).map((key) => {
        preferenceSetting[key] = data?.[key] ?? defaultPreferenceSetting[key]
      })
      this.preferenceSetting = preferenceSetting
    },

    async getConfig() {
      const res = await getConfig({
        info: true
      })
      this.controlConfig = res?.data || {}
    },

    initSortable() {
      const dragTab = document.getElementById('workstation-drag-tab')?.querySelector?.('.ant-tabs-nav')?.firstChild
      if (dragTab) {
        this.sortableInstance = Sortable.create(dragTab, {
          handle: '.ant-tabs-tab', // css selector
          draggable: '.ant-tabs-tab:not(:first-child)', // draggable css selector
          onEnd: this.handleSortEnd
        })
      }
    },

    updateSelectedKeys(keys) {
      this.selectedKeys = keys
    },

    async openTerminal(data) {
      const id = uuidv4()
      const accountName = this.getAccountName(data.accountId)
      const name = accountName ? `${accountName}@${data.assetName}` : data.assetName
      const permissions = await this.getAssetPermissions(data.assetId, data.accountId)

      this.terminalList.push({
        ...data,
        socketStatus: SOCKET_STATUS.LOADING,
        id,
        name,
        type: this.getConnectType(data.protocolType),
        permissions: permissions?.[data.accountId] || {}
      })

      this.tabActiveKey = id
    },

    async openTerminalList(data) {
      const permissions = await this.getAssetPermissions(data.assetId, data.accountList.map((id) => id).join(','))

      const newList = data.accountList.map((id) => {
        const accountName = this.getAccountName(id)
        const name = accountName ? `${accountName}@${data.assetName}` : data.assetName

        return {
          protocolType: data.protocolType,
          protocol: data.protocol,
          assetId: data.assetId,
          name,
          accountId: id,
          socketStatus: SOCKET_STATUS.LOADING,
          id: uuidv4(),
          type: this.getConnectType(data.protocolType),
          permissions: permissions?.[id] || {}
        }
      })

      this.tabActiveKey = newList[0].id
      this.terminalList.push(...newList)
    },

    async getAssetPermissions(assetId, account_ids) {
      const defaultPermissions = this.controlConfig?.default_permissions
      const permissions = {}

      try {
        const res = await getAssetPermissions(assetId, { account_ids })
        const data = res?.data?.results || {}
        Object.keys(data).forEach((accountId) => {
          const permissionData = data?.[accountId]?.results || {}
          permissions[accountId] = {}
          Object.keys(defaultPermissions).forEach((permissionType) => {
            permissions[accountId][permissionType] = permissionData?.[permissionType]?.allowed ?? defaultPermissions?.[permissionType] ?? false
          })
        })
      } catch (error) {
        console.error('getAssetPermissions error', error)
      }
      return permissions
    },

    closeTerminal(item, index) {
      if (item.id === this.tabActiveKey) {
        this.tabActiveKey = index === 0 ? WORKSTATION_TAB_TYPE.MY_ASSETS : this.terminalList[index - 1].id
      }
      this.terminalList.splice(index, 1)
      this.getOfUserStat()
    },

    copyTerminal(item) {
      const id = uuidv4()
      this.terminalList.push({
        ...item,
        socketStatus: SOCKET_STATUS.LOADING,
        id
      })

      this.tabActiveKey = id
    },

    getConnectType(protocolType) {
      if (['ssh', 'telnet', 'mysql', 'redis', 'postgresql', 'mongodb'].includes(protocolType)) {
        return WORKSTATION_TAB_TYPE.TERMINAL
      } else {
        return WORKSTATION_TAB_TYPE.GUACAMOLE
      }
    },

    handleTerminalSocketStatus(item, status) {
      const terminalIndex = this.terminalList.findIndex((terminal) => terminal.id === item.id)
      if (terminalIndex > -1) {
        this.terminalList[terminalIndex].socketStatus = status
        this.getOfUserStat()
      }
    },

    openRecentSession() {
      this.$refs.recentSessionRef.open()
    },

    openSystemSetting(type) {
      const findData = this.terminalList.find((item) => item.type === type)
      if (findData) {
        this.tabActiveKey = findData.id
      } else {
        let name = ''
        switch (type) {
          case WORKSTATION_TAB_TYPE.DISPLAY_SETTING:
            name = 'oneterm.terminalDisplay.displaySetting'
            break
          case WORKSTATION_TAB_TYPE.THEME_SETTING:
            name = 'oneterm.terminalDisplay.themeSetting'
            break
          default:
            break
        }

        const id = uuidv4()
        this.terminalList.push({
          id,
          type,
          name: this.$t(name)
        })

        this.tabActiveKey = id
      }
    },

    handleSortEnd(evt) {
      const { oldIndex, newIndex } = evt
      if (oldIndex === newIndex) {
        return
      }

      const terminalList = [...this.terminalList]
      const movedItem = terminalList.splice(oldIndex - 1, 1)[0]
      terminalList.splice(newIndex - 1, 0, movedItem)
      this.terminalList = terminalList
    },

    openBatchExecution(data) {
      const id = uuidv4()
      this.terminalList.push({
        id,
        type: WORKSTATION_TAB_TYPE.BATCH_EXECUTION,
        name: this.$t('oneterm.workStation.batchExecution'),
        batchExecutionData: data
      })
      this.tabActiveKey = id
    },

    callComponentFn(fnName) {
      const component = this.$refs?.[`workStationPanelRef${this.tabActiveKey}`]?.[0]
      if (component?.[fnName]) {
        component[fnName]()
      }
    },

    toggleOperationMenu() {
      this.showOperationMenu = !this.showOperationMenu
      localStorage.setItem(operationMenuExpandKey, this.showOperationMenu)
    },

    getAccountName(id) {
      if (!id) {
        return ''
      }

      return this.accountList.find((account) => account.id === id)?.name || ''
    }
  },
}
</script>

<style lang="less" scoped>
.oneterm-workstation {
  width: 100%;
  height: 100%;

  &-two {
    width: 100%;
    height: 100%;
    background-color: #FFFFFF;
    border-radius: 15px;
    display: flex;

    /deep/ .ant-tabs {
      flex-grow: 1;
      padding: 0px 18px 18px;
    }

    .oneterm-workstation-panel {
      height: calc(100vh - 172px);
      margin: 0px;
      background-color: #FFFFFF;
    }

    &_full {
      position: fixed;
      top: 0;
      left: 0;
      width: 100vw;
      height: 100vh;
      z-index: 1000;

      .oneterm-workstation-panel {
        height: calc(100vh - 100px);
      }
    }
  }

  .oneterm-workstation-tab-terminal {
    display: flex;
    align-items: center;
    column-gap: 3px;

    &-status {
      width: 12px;
      height: 12px;
      border-radius: 50%;
      background-color: #00B42A22;
      position: relative;

      &::before {
        content: '';
        position: absolute;
        top: 50%;
        left: 50%;
        width: 6px;
        height: 6px;
        border-radius: 50%;
        margin-top: -3px;
        margin-left: -3px;
        background-color: #00B42A;
      }

      &_error {
        background-color: #F2637B22;

        &::before {
          background-color: #F2637B;
        }
      }
    }

    &-title {
      max-width: 200px;
      overflow: hidden;
      text-overflow: ellipsis;
      text-wrap: nowrap;
    }

    &-icon {
      font-size: 12px;
      color: #A5A9BC;
      cursor: pointer;
      opacity: 0;
      margin: 0px;
    }
  }

  /deep/ .ant-tabs-tab {
    padding: 12px 8px;

    &:hover {
      .oneterm-workstation-tab-terminal-icon {
        opacity: 1;
      }
    }
  }

  .operation-menu-icon {
    font-size: 18px;
  }
}
</style>
