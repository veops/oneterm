<template>
  <div class="oneterm-dashboard">
    <div class="oneterm-dashboard-top">
      <div v-for="(sta, index) in statistics" :key="sta.label">
        <div class="oneterm-statistic-img">
          <img :src="require(`../../assets/dashboard-${index + 1}.png`)" />
        </div>
        <div class="oneterm-statistic-container">
          <div>{{ $t(`oneterm.dashboard.${sta.label}`) }}</div>
          <div>
            <strong>{{ sta.value }}</strong>
          </div>
        </div>
      </div>
    </div>
    <div class="oneterm-dashboard-center">
      <AssetActive></AssetActive>
      <AssetType></AssetType>
    </div>
    <div class="oneterm-dashboard-bottom">
      <Account></Account>
      <UserRank></UserRank>
    </div>
  </div>
</template>

<script>
import { mapState } from 'vuex'
import AssetActive from './assetActive.vue'
import AssetType from './assetType.vue'
import Account from './account.vue'
import UserRank from './userRank.vue'
import { getCountStat } from '../../api/stat'
export default {
  name: 'Dashboard',
  components: { AssetActive, AssetType, Account, UserRank },
  data() {
    return {
      statistics: [],
    }
  },
  computed: {
    ...mapState({
      allUsers: (state) => state.user.allUsers,
    }),
  },
  mounted() {
    this.getCountStat()
  },
  methods: {
    getCountStat() {
      getCountStat().then((res) => {
        const { asset, connect, gateway, session, total_asset, total_gateway, user } = res?.data ?? {}
        this.statistics = [
          {
            label: 'currentConnect',
            value: connect,
          },
          {
            label: 'currentSession',
            value: session,
          },
          {
            label: 'assetsInOperation',
            value: `${asset}/${total_asset}`,
          },
          {
            label: 'currentUsers',
            value: `${user}/${this.allUsers.filter((item) => !item.block).length}`,
          },
          {
            label: 'currentGateways',
            value: `${gateway}/${total_gateway}`,
          },
        ]
      })
    },
  },
}
</script>

<style lang="less">
.dashbboard-layout {
  padding: 18px;
  position: relative;
  display: flex;
  flex-direction: column;
  h4 {
    color: black;
    font-size: 1vw;
    font-weight: 700;
  }
  .dashboard-timeradio {
    position: absolute;
    right: 18px;
    top: 18px;
  }
}
</style>

<style lang="less" scoped>
.oneterm-dashboard {
  width: 100%;
  height: calc(100vh - 88px);
  display: grid;
  grid-gap: 22px 0;
  grid-template-columns: 100%;
  grid-template-rows: 15vh 1fr 33vh;
  .oneterm-dashboard-top {
    background-color: #fff;
    border-radius: 5px;
    display: flex;
    > div {
      width: 20%;
      height: 100%;
      display: flex;
      position: relative;
      &:not(:last-child)::after {
        content: '';
        position: absolute;
        width: 1px;
        height: 60%;
        background-color: #f0f1f5;
        right: 0;
        top: 20%;
      }
      .oneterm-statistic-img {
        width: 40%;
        display: flex;
        align-items: center;
        justify-content: center;
        img {
          width: 65%;
          max-width: 80px;
        }
      }
      .oneterm-statistic-container {
        flex: 1;
        display: flex;
        flex-direction: column;
        justify-content: center;
        > div:first-child {
          font-size: 16px;
          color: #9094a6;
          font-weight: 500;
        }
        strong {
          font-size: 28px;
          color: #000;
          font-weight: 700;
        }
      }
    }
  }
  .oneterm-dashboard-center,
  .oneterm-dashboard-bottom {
    display: grid;
    grid-gap: 0 24px;
    grid-template-columns: 1fr 397px;
    grid-template-rows: 100%;
    > div {
      background-color: #fff;
      border-radius: 5px;
    }
  }
}
</style>
