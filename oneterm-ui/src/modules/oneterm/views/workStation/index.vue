<template>
  <div class="oneterm-workstation">
    <div class="oneterm-left">
      <div class="oneterm-asset">
        <div class="oneterm-workstation-header">
          <a-space>
            <ops-icon type="oneterm-myassets" />
            <strong>{{ $t('oneterm.workStation.myAsset') }}</strong>
            <a-icon :type="expandKeys.includes('asset') ? 'caret-down' : 'caret-up'" @click="toggle('asset')" />
          </a-space>
        </div>
        <div class="oneterm-layout" v-show="expandKeys.includes('asset')">
          <AssetList
            class="oneterm-workstation-myasset"
            :style="{
              '--height':
                expandKeys.includes('asset') && expandKeys.length === 1
                  ? 'calc(100vh - 40px - 48px - 24px - 33px - 33px)'
                  : 'calc((100vh - 40px - 48px) / 2 - 12px - 33px)',
            }"
            :forMyAsset="true"
          />
        </div>
      </div>
      <div class="oneterm-session">
        <div class="oneterm-workstation-header">
          <a-space>
            <ops-icon type="oneterm-recentsession" />
            <strong>{{ $t('oneterm.workStation.recentSession') }}</strong>
            <a-icon :type="expandKeys.includes('session') ? 'caret-down' : 'caret-up'" @click="toggle('session')" />
          </a-space>
        </div>
        <RecentSession
          v-show="expandKeys.includes('session')"
          :style="{
            height:
              expandKeys.includes('session') && expandKeys.length === 1
                ? 'calc(100vh - 40px - 48px - 24px - 33px - 33px)'
                : 'calc((100vh - 40px - 48px) / 2 - 12px - 33px)',
          }"
        />
      </div>
    </div>
    <div class="oneterm-workstation-info">
      <div>
        <strong>{{ $t('oneterm.workStation.personalInfo') }}</strong>
        <div class="oneterm-workstation-info-box">
          <a-avatar
            v-if="personAvatar"
            :size="48"
            :src="personAvatar.startsWith('https') ? personAvatar : `/api/common-setting/v1/file/${personAvatar}`"
          />
          <a-avatar v-else :style="{ backgroundColor: '#2F54EB', fontSize: '12px' }" :size="48">
            {{ personName.substring(0, 1) }}
          </a-avatar>
          <div>
            <div>
              <strong>{{ personName }}</strong>
            </div>
            <span><a-icon type="user" />{{ isOnetermAdmin ? $t(`admin`) : $t(`user`) }}</span>
          </div>
        </div>
        <div class="oneterm-workstation-info-asset" v-for="item in personInfoList" :key="item.valueKey">
          <a-space>
            <ops-icon :type="`oneterm-${item.valueKey}`" />
            <span style="color:#86909C">{{ $t(`oneterm.${item.label}`) }}</span>
          </a-space>
          <strong>{{ personInfo[item.valueKey] }}</strong>
        </div>
      </div>

      <a-divider v-if="last_login">{{ $t(`oneterm.workStation.loginTime`) }}：{{ last_login }}</a-divider>
    </div>
  </div>
</template>

<script>
import { mapState } from 'vuex'
import { getOfUserStat } from '../../api/stat'
import RecentSession from './recentSession.vue'
import AssetList from '../../views/assets/assets/assetList.vue'
export default {
  name: 'WorkStation',
  components: { RecentSession, AssetList },
  data() {
    const personInfoList = [
      {
        label: 'session',
        valueKey: 'session',
      },
      {
        label: 'connect',
        valueKey: 'connect',
      },
      {
        label: 'connectedAssets',
        valueKey: 'asset',
      },
      {
        label: 'totalAssets',
        valueKey: 'total_asset',
      },
    ]
    return {
      personInfoList,
      personInfo: {},
      expandKeys: ['session', 'asset'],
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
      return this.personRoles.permissions.includes('oneterm_admin')
    },
  },
  mounted() {
    getOfUserStat().then((res) => {
      this.personInfo = res?.data ?? {}
    })
  },
  methods: {
    toggle(key) {
      const _idx = this.expandKeys.findIndex((item) => item === key)
      if (_idx > -1) {
        this.expandKeys.splice(_idx, 1)
      } else {
        this.expandKeys.push(key)
      }
    },
  },
}
</script>

<style lang="less" scoped>
@import '~@/style/static.less';
.oneterm-workstation {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  .oneterm-left {
    width: calc(100% - 320px - 24px);
    height: 100%;
    margin-right: 24px;
    .oneterm-session,
    .oneterm-asset {
      background-color: #fff;
      border-radius: 5px;
    }
    .oneterm-asset {
      margin-bottom: 24px;
    }
    .oneterm-workstation-header {
      padding: 6px 12px;
      position: relative;
      &::after {
        content: '';
        position: absolute;
        width: 100%;
        height: 1px;
        background-color: #e4e7ed;
        left: 0;
        bottom: 0;
      }
      i {
        color: #a5a9bc;
        cursor: pointer;
        &:hover {
          color: @primary-color;
        }
      }
    }
  }

  .oneterm-workstation-info {
    width: 320px;
    height: calc((100vh - 40px - 48px) / 2 - 12px);
    background-color: #fff;
    padding: 6px 12px;
    overflow-y: auto;
    position: relative;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    border-radius: 5px;
    .oneterm-workstation-info-box {
      display: flex;
      flex-direction: row;
      width: 100%;
      background-color: #f0f2f9;
      border-radius: 4px;
      padding: 24px 18px;
      align-items: center;
      margin: 12px 0;
      > div {
        flex: 1;
        padding: 0 24px;
        strong {
          color: #4e5969;
        }
        > div {
          margin-bottom: 12px;
        }
        > span {
          color: @primary-color;
          background-color: #e9eeff;
          border-radius: 30px;
          border: 1px solid #c9d6f8;
          padding: 3px 12px;
          i {
            margin-right: 5px;
          }
        }
      }
    }
    .oneterm-workstation-info-asset {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 44px;
      position: relative;
      i {
        font-size: 26px;
      }
      strong {
        font-size: 18px;
      }
      &::after {
        content: '';
        position: absolute;
        width: 100%;
        height: 1px;
        bottom: 0;
        left: 0;
        background-color: #f0f1f5;
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
