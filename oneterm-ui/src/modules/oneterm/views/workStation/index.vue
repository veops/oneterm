<template>
  <div class="oneterm-workstation">
    <div class="oneterm-workstation-info">
      <div class="oneterm-workstation-info-box">
        <span class="oneterm-workstation-info-box-tag">
          <ops-icon type="monitor-director" class="oneterm-workstation-info-box-tag-icon" />
          <span class="oneterm-workstation-info-box-tag-text">{{ isOnetermAdmin ? $t(`admin`) : $t(`user`) }}</span>
        </span>
        <div class="oneterm-workstation-info-box-name" >{{ personName }}</div>
      </div>
      <div class="oneterm-workstation-info-asset" v-for="item in personInfoList" :key="item.valueKey">
        <ops-icon :type="item.icon" />
        <span>{{ $t(`oneterm.${item.label}`) }}:</span>
        <strong>{{ personInfo[item.valueKey] }}</strong>
      </div>
    </div>
    <div class="oneterm-left">
      <a-tabs v-model="tabActiveKey">
        <a-tab-pane key="1">
          <template #tab>
            <div>
              <ops-icon type="veops-resource2" />
              <strong>{{ $t('oneterm.workStation.myAsset') }}</strong>
            </div>
          </template>

          <div class="oneterm-layout">
            <AssetList
              :forMyAsset="true"
              class="oneterm-workstation-data"
              @openTerminal="openTerminal"
              @openTerminalList="openTerminalList"
            />
          </div>
        </a-tab-pane>

        <a-tab-pane key="2">
          <template #tab>
            <div>
              <ops-icon type="oneterm-recentsession" />
              <strong>{{ $t('oneterm.workStation.recentSession') }}</strong>
            </div>
          </template>

          <div class="oneterm-layout">
            <RecentSession
              v-if="tabActiveKey === '2'"
              class="oneterm-workstation-data"
              @openTerminal="openTerminal"
            />
          </div>
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
              v-if="item.protocolType === 'ssh'"
              :assetId="item.assetId"
              :accountId="item.accountId"
              :protocol="item.protocol"
              :isFullScreen="false"
              class="oneterm-workstation-data"
              @close="handleTerminalError(item)"
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
    </div>
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
    const personInfoList = [
      {
        label: 'session',
        valueKey: 'session',
        icon: 'oneterm-session1'
      },
      {
        label: 'connect',
        valueKey: 'connect',
        icon: 'oneterm-connect1'
      },
      {
        label: 'connectedAssets',
        valueKey: 'asset',
        icon: 'oneterm-assets'
      },
      {
        label: 'totalAssets',
        valueKey: 'total_asset',
        icon: 'veops-resource2'
      },
    ]
    return {
      personInfoList,
      personInfo: {},
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
          this.personInfo = res?.data ?? {}
        }, delayTime)
      } else {
        const res = await getOfUserStat({
            info: true
          })
        this.personInfo = res?.data ?? {}
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
      this.getOfUserStat(1000)
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
      this.getOfUserStat(1000)
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
      this.getOfUserStat(1000)
    },

    handleTerminalError(item) {
      const terminalIndex = this.terminalList.findIndex((terminal) => terminal.id === item.id)
      if (terminalIndex > -1) {
        this.terminalList[terminalIndex].socketStatus = false
        this.getOfUserStat(1000)
      }
    }
  },
}
</script>

<style lang="less" scoped>
.oneterm-workstation {
  width: 100%;
  height: 100%;

  .oneterm-left {
    width: 100%;
    // margin-top: 20px;
    background-color: #FFFFFF;
    border-radius: 2px;

    /deep/ .ant-tabs-bar {
      margin-bottom: 0px;
      margin-left: 18px;
      margin-right: 18px;
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

  .oneterm-workstation-info {
    width: 100%;
    height: 40px;
    // background-color: rgba(255, 255, 255, 0.70);
    padding: 0px 20px;
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    border-radius: 2px;

    .oneterm-workstation-info-box {
      display: flex;
      align-items: center;
      width: 25%;

      &-tag {
        height: 23px;
        padding: 0 10px;
        border-radius: 100px;
        border: solid 1px #C9D6F8;
        background-color: #E9EEFF;

        &-tag {
          font-size: 10px;
          color: #84A4F9;
        }

        &-text {
          margin-left: 3px;
          font-size: 12px;
          font-weight: 400;
          color: #2156DF;
        }
      }

      &-name {
        margin-left: 10px;
        font-size: 15px;
        font-weight: 500;
        color: #1D2129;
      }
    }
    .oneterm-workstation-info-asset {
      display: flex;
      align-items: center;
      width: 25%;

      i {
        font-size: 18px;
      }

      span {
        margin-left: 8px;
        font-size: 14px;
        font-weight: 400;
        color: #4E5969;
      }

      strong {
        margin-left: 8px;
        font-size: 16px;
        font-weight: 500;
        color: #1D2129;
      }
    }
  }
}
</style>

<style lang="less">
.oneterm-workstation-info {
  .ant-divider {
    margin: 8px 0;
  }
  .ant-divider-inner-text {
    color: #86909c;
    font-size: 14px;
  }
}
.two-column-layout.oneterm-asset-list.oneterm-workstation-myasset {
  height: var(--height) !important;
  margin-bottom: 0;
}
</style>
