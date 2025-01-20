<template>
  <div class="oneterm-workstation oneterm-layout">
    <AssetList
      :forMyAsset="true"
      :userStat="userStat"
      :showAssetTable="tabActiveKey === '1'"
      @openTerminal="openTerminal"
      @openTerminalList="openTerminalList"
    >
      <template #title>
        <div class="asset-tree-title">
          <div class="asset-tree-title-text">
            {{ $t('oneterm.assetList.assetTree') }}
          </div>

          <a-button
            type="primary"
            class="ops-button-ghost"
            ghost
            size="small"
            @click="openRecentSession"
          >
            {{ $t('oneterm.workStation.viewRecentSession') }}
          </a-button>
        </div>
      </template>

      <template #two-tab>
        <a-tabs
          v-model="tabActiveKey"
        >
          <a-tab-pane key="1">
            <template #tab>
              <div>
                <ops-icon type="veops-resource2" />
                <strong>{{ $t('oneterm.workStation.myAsset') }}</strong>
              </div>
            </template>
            <!-- tab1 在 AssetLIst 内部展示 -->
          </a-tab-pane>

          <template v-if="terminalList.length" >
            <a-tab-pane
              v-for="(item, index) in terminalList"
              :key="item.id"
            >
              <template #tab>
                <div class="oneterm-workstation-tab-terminal">
                  <span
                    :class="['oneterm-workstation-tab-terminal-status', !item.socketStatus ? 'oneterm-workstation-tab-terminal-status_error' : '']"
                  ></span>

                  <a-tooltip :title="item.assetName">
                    <span class="oneterm-workstation-tab-terminal-title">{{ item.assetName }}</span>
                  </a-tooltip>

                  <a-icon
                    class="oneterm-workstation-tab-terminal-icon"
                    type="close"
                    @click.stop="closeTerminal(item, index)"
                  />
                  <ops-icon
                    class="oneterm-workstation-tab-terminal-icon"
                    type="veops-copy"
                    @click.stop="copyTerminal(item)"
                  />
                </div>
              </template>

              <TerminalPanel
                v-if="['ssh'].includes(item.protocolType)"
                :assetId="item.assetId"
                :accountId="item.accountId"
                :protocol="item.protocol"
                :isFullScreen="false"
                class="oneterm-workstation-data"
                @close="handleTerminalError(item)"
                @open="getOfUserStat(1000)"
              />

              <GuacamolePanel
                v-else
                :assetId="item.assetId"
                :accountId="item.accountId"
                :protocol="item.protocol"
                :isFullScreen="false"
                class="oneterm-workstation-data"
                @close="handleTerminalError(item)"
              />
            </a-tab-pane>
          </template>
        </a-tabs>
      </template>
    </AssetList>

    <RecentSession
      ref="recentSessionRef"
      @openTerminal="openTerminal"
    />
  </div>
</template>

<script>
import { v4 as uuidv4 } from 'uuid'
import { mapState } from 'vuex'
import { getOfUserStat } from '../../api/stat'
import RecentSession from './recentSession.vue'
import AssetList from '../../views/assets/assets/assetList.vue'
import TerminalPanel from '../terminal/index.vue'
import GuacamolePanel from '../terminal/guacamoleClient.vue'

export default {
  name: 'WorkStation',
  components: { RecentSession, AssetList, TerminalPanel, GuacamolePanel },
  data() {
    return {
      userStat: {},
      expandKeys: ['session', 'asset'],
      terminalList: [],
      tabActiveKey: '1',
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
  },
  mounted() {
    this.getOfUserStat()
  },
  methods: {
    async getOfUserStat(delayTime) {
      if (delayTime) {
        setTimeout(async () => {
          const res = await getOfUserStat({
            info: true
          })
          this.userStat = res?.data ?? {}
        }, delayTime)
      } else {
        const res = await getOfUserStat({
            info: true
          })
        this.userStat = res?.data ?? {}
      }
    },

    toggle(key) {
      const _idx = this.expandKeys.findIndex((item) => item === key)
      if (_idx > -1) {
        this.expandKeys.splice(_idx, 1)
      } else {
        this.expandKeys.push(key)
      }
    },

    openTerminal(data) {
      const id = uuidv4()
      this.terminalList.push({
        ...data,
        socketStatus: true,
        id
      })

      this.tabActiveKey = id
    },

    openTerminalList(data) {
      const newList = data.accountList.map((id) => {
        return {
          protocolType: data.protocolType,
          protocol: data.protocol,
          assetId: data.assetId,
          assetName: data.assetName,
          accountId: id,
          socketStatus: true,
          id: uuidv4()
        }
      })
      this.tabActiveKey = newList[0].id
      this.terminalList.push(...newList)
    },

    closeTerminal(item, index) {
      if (item.id === this.tabActiveKey) {
        this.tabActiveKey = index === 0 ? '1' : this.terminalList[index - 1].id
      }
      this.terminalList.splice(index, 1)
      this.getOfUserStat(1000)
    },

    copyTerminal(item) {
      const id = uuidv4()
      this.terminalList.push({
        ...item,
        socketStatus: true,
        id
      })

      this.tabActiveKey = id
    },

    handleTerminalError(item) {
      const terminalIndex = this.terminalList.findIndex((terminal) => terminal.id === item.id)
      if (terminalIndex > -1) {
        this.terminalList[terminalIndex].socketStatus = false
        this.getOfUserStat(1000)
      }
    },

    openRecentSession() {
      this.$refs.recentSessionRef.open()
    }
  },
}
</script>

<style lang="less" scoped>
.oneterm-workstation {
  width: 100%;
  height: 100%;

  .asset-tree-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    column-gap: 12px;
    row-gap: 6px;

    &-text {
      padding-left: 12px;
      border-left: solid 3px @primary-color;
      font-size: 15px;
      font-weight: 700;
      color: #000000;
      flex-shrink: 0;
    }
  }

  /deep/ .oneterm-layout-container {
    padding-top: 0px;
  }

  .oneterm-workstation-data {
    height: calc(100vh - 172px) !important;
    margin: 0px;
    background-color: #FFFFFF;
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
    &:hover {
      .oneterm-workstation-tab-terminal-icon {
        opacity: 1;
      }
    }
  }
}
</style>
