<template>
  <div class="dashbboard-layout">
    <h4>{{ $t('oneterm.dashboard.userRank') }}</h4>
    <div class="user-rank">
      <template v-for="(item, index) in rankList">
        <div class="user-rank-box" :key="item.uid">
          <a-avatar
            class="user-rank-box-avatar"
            v-if="getUserKeyByUid(item.uid, 'avatar')"
            :size="36"
            :src="
              getUserKeyByUid(item.uid, 'avatar').startsWith('https')
                ? getUserKeyByUid(item.uid, 'avatar')
                : `/api/common-setting/v1/file/${getUserKeyByUid(item.uid, 'avatar')}`
            "
          />
          <a-avatar
            v-else
            class="user-rank-box-avatar user-rank-box-avatar-default"
            :size="36"
          >
            {{ getUserKeyByUid(item.uid).substring(0, 1) }}
          </a-avatar>
          <div class="user-rank-box-center">
            <a-tooltip :title="getUserKeyByUid(item.uid)" placement="topLeft" >
              <div class="user-rank-box-name">{{ getUserKeyByUid(item.uid) }}</div>
            </a-tooltip>
            <div class="user-rank-box-time">{{ moment(item.last_time).format('YYYY-MM-DD HH:mm:ss') }}</div>
          </div>
          <div class="user-rank-box-count">
            <span>{{ item.count > 999 ? '999+' : item.count }}{{ $t('oneterm.dashboard.userRankTimes') }}</span>
          </div>
        </div>
        <div v-if="index < rankList.length - 1" class="user-rank-box-divider" :key="`divider-${item.uid}`"></div>
      </template>
    </div>
  </div>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import { getRankOfUserStat } from '../../api/stat'
export default {
  name: 'UserRank',
  data() {
    return {
      rankList: [],
    }
  },
  computed: {
    ...mapState({
      allUsers: (state) => state.user.allUsers,
    }),
  },
  mounted() {
    getRankOfUserStat().then((res) => {
      this.rankList = res?.data?.list ?? []
    })
  },
  methods: {
    moment,
    getUserKeyByUid(uid, key = 'nickname') {
      const _find = this.allUsers.find((user) => user.uid === uid)
      return _find?.[key] ?? ''
    },
  },
}
</script>

<style lang="less" scoped>
.user-rank {
  display: flex;
  flex-direction: column;
  overflow: auto;
  .user-rank-box-divider {
    width: 100%;
    height: 1px;
    background-color: #f0f1f5;
  }
  .user-rank-box {
    width: 100%;
    display: flex;
    flex-direction: row;
    align-items: center;
    padding: 10px 0;
    position: relative;

    &-avatar {
      flex-shrink: 0;

      &-default {
        background-color: @primary-color;
        font-size: 12px;
      }
    }

    &-center {
      margin-left: 16px;
      margin-right: 16px;
      max-width: 100%;
      overflow: hidden;
    }

    .user-rank-box-name {
      color: #252631;
      font-weight: 400;
      font-size: 14px;
      overflow: hidden;
      text-wrap: nowrap;
      text-overflow: ellipsis;
      max-width: 100%;
    }
    .user-rank-box-time {
      color: #98a9bc;
      font-weight: 400;
      font-size: 14px;
    }
    .user-rank-box-count {
      margin-left: auto;
      flex-shrink: 0;

      & > span {
        color: @primary-color;
        background-color: #dce3fb;
        padding: 0 10px;
        border-radius: 1px;
      }
    }
  }
}
</style>
