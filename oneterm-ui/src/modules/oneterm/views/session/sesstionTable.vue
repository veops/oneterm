<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">
      <a-icon
        v-if="isDetail"
        type="left"
        @click="
          () => {
            isDetail = false
          }
        "
      />
      {{ status === 1 ? $t('oneterm.menu.onlineSession') : $t('oneterm.menu.offlineSession')
      }}{{ isDetail ? `：${$t('oneterm.menu.commandRecord')}` : '' }}
    </div>
    <div class="oneterm-layout-container">
      <div v-show="!isDetail">
        <div class="oneterm-layout-container-header">
          <a-input-search
            allow-clear
            v-model="filterName"
            :style="{ width: '250px' }"
            class="ops-input ops-input-radius"
            :placeholder="$t('placeholderSearch')"
            @search="updateTableData()"
          />
          <a-space>
            <div class="ops-list-batch-action" v-show="!!selectedRowKeys.length">
              <span @click="exportTable">{{ $t('export') }}</span>
              <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
            </div>
            <a-button
              @click="
                () => {
                  selectedRowKeys = []
                  $refs.opsTable.getVxetableRef().clearCheckboxRow()
                  $refs.opsTable.getVxetableRef().clearCheckboxReserve()
                  updateTableData()
                }
              "
            >{{ $t(`refresh`) }}</a-button
            >
          </a-space>
        </div>
        <ops-table
          size="small"
          ref="opsTable"
          stripe
          class="ops-stripe-table"
          :data="tableData"
          show-overflow
          show-header-overflow
          @checkbox-change="onSelectChange"
          @checkbox-all="onSelectChange"
          @checkbox-range-end="onSelectRangeEnd"
          :checkbox-config="{ reserve: true, highlight: true, range: true }"
          :row-config="{ keyField: 'id' }"
          :height="tableHeight"
          resizable
          :loading="loading"
        >
          <vxe-column type="checkbox" width="60px"></vxe-column>
          <vxe-column :title="$t(`user`)" field="user_name"> </vxe-column>
          <vxe-column :title="$t(`oneterm.asset`)" field="asset_info"> </vxe-column>
          <vxe-column :title="$t(`oneterm.gateway`)" field="gateway_info"> </vxe-column>
          <vxe-column :title="$t(`oneterm.account`)" field="account_info"> </vxe-column>
          <vxe-column :title="$t('oneterm.protocol')" field="protocol"> </vxe-column>
          <vxe-column :title="$t(`oneterm.sessionTable.clientIp`)" field="client_ip"> </vxe-column>
          <vxe-column :title="$t(`oneterm.sessionTable.cmdCount`)" field="cmd_count"> </vxe-column>
          <vxe-column :title="$t('created_at')" field="created_at" width="150px">
            <template #default="{row}">
              {{ moment(row.created_at).format('YYYY-MM-DD HH:mm:ss') }}
            </template>
          </vxe-column>
          <vxe-column :title="$t(`oneterm.sessionTable.duration`)" field="duration">
            <template #default="{row}">
              {{ calcDuration(row.duration) }}
            </template>
          </vxe-column>
          <vxe-column :title="$t(`operation`)" width="100" align="center">
            <template #default="{row}">
              <a-space>
                <template v-if="status === 2">
                  <a-tooltip :title="$t('oneterm.sessionTable.replay')">
                    <a @click="openReplay(row)"><ops-icon type="oneterm-playback"/></a>
                  </a-tooltip>
                  <a-tooltip :title="$t('download')">
                    <a :href="`/api/oneterm/v1/session/replay/${row.session_id}`"><a-icon type="download"/></a>
                  </a-tooltip>
                  <a-tooltip :title="$t('oneterm.menu.commandRecord')">
                    <a @click="openDetail(row)"><ops-icon type="oneterm-command_record"/></a>
                  </a-tooltip>
                </template>
                <template v-else>
                  <a-tooltip :title="$t('oneterm.sessionTable.monitor')">
                    <a @click="openMonitor(row)"><a-icon type="eye"/></a>
                  </a-tooltip>
                  <a-tooltip :title="$t('oneterm.sessionTable.disconnect')">
                    <a-popconfirm :title="$t('oneterm.sessionTable.confirmDisconnect')" @confirm="disconnect(row)">
                      <a><ops-icon type="oneterm-disconnect"/></a>
                    </a-popconfirm>
                  </a-tooltip>
                </template>
              </a-space>
            </template>
          </vxe-column>
        </ops-table>
        <div class="oneterm-layout-pagination">
          <a-pagination
            size="small"
            show-size-changer
            :current="tablePage.currentPage"
            :total="tablePage.totalResult"
            :show-total="
              (total, range) =>
                $t('pagination.total', {
                  range0: range[0],
                  range1: range[1],
                  total,
                })
            "
            :page-size="tablePage.pageSize"
            :default-current="1"
            @change="pageOrSizeChange"
            @showSizeChange="pageOrSizeChange"
          />
        </div>
      </div>
      <SessionDetailTable v-if="isDetail" :session_id="session_id" />
    </div>
  </div>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import { getSessionList } from '../../api/session'
import SessionDetailTable from './sessionDetailTable.vue'
import { closeConnect } from '../../api/connect'
import { initMessageStorageKey } from '../terminal/index.vue'

export default {
  name: 'SessionTable',
  components: { SessionDetailTable },
  props: {
    status: {
      type: Number,
      default: 1,
    },
  },
  data() {
    return {
      filterName: '',
      tableData: [],
      tablePage: {
        currentPage: 1,
        pageSize: 20,
        totalResult: 0,
      },
      selectedRowKeys: [],
      loading: false,
      isDetail: false,
      session_id: null,
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
    }),
    tableHeight() {
      return this.windowHeight - 258
    },
  },
  mounted() {
    this.updateTableData()
  },
  methods: {
    moment,
    updateTableData(currentPage = 1, pageSize = this.tablePage.pageSize) {
      this.loading = true
      getSessionList({
        page_index: currentPage,
        page_size: pageSize,
        search: this.filterName,
        status: this.status,
      })
        .then((res) => {
          this.tableData = res?.data?.list || []
          this.tablePage = {
            ...this.tablePage,
            currentPage,
            pageSize,
            totalResult: res?.data?.count ?? 0,
          }
        })
        .finally(() => {
          this.loading = false
        })
    },
    onSelectChange() {
      const opsTable = this.$refs.opsTable.getVxetableRef()
      const records = [...opsTable.getCheckboxRecords(), ...opsTable.getCheckboxReserveRecords()]
      this.selectedRowKeys = records.map((i) => i.id)
    },
    onSelectRangeEnd({ records }) {
      this.selectedRowKeys = records.map((i) => i.id)
    },
    pageOrSizeChange(currentPage, pageSize) {
      this.updateTableData(currentPage, pageSize)
    },
    openDetail(row) {
      this.session_id = row.session_id
      this.$nextTick(() => {
        this.isDetail = true
      })
    },
    calcDuration(duration) {
      const h = parseInt((duration / 60 / 60) % 24)
      const m = parseInt((duration / 60) % 60)
      const s = parseInt(duration % 60)
      let time = ''
      if (h) {
        time = `${time}${h}${this.$t('oneterm.sessionTable.hour')}`
      }
      if (m) {
        time = `${time}${m}${this.$t('oneterm.sessionTable.minute')}`
      }
      if (s) {
        time = `${time}${s}${this.$t('oneterm.sessionTable.second')}`
      }
      return time
    },
    openReplay(row) {
      if (row.protocol.includes('rdp') || row.protocol.includes('vnc')) {
        window.open(`/oneterm/replay/guacamole/${row.session_id}`, '_blank')
      } else {
        window.open(`/oneterm/replay/${row.session_id}`, '_blank')
      }
    },
    openMonitor(row) {
      if (row.protocol.includes('rdp') || row.protocol.includes('vnc')) {
        const { asset_id, account_id, protocol } = row
        window.open(
          `/oneterm/guacamole/${asset_id}/${account_id}/${protocol}?session_id=${row.session_id}&is_monitor=true`,
          '_blank'
        )
      } else {
        const dataList = [
          {
            key: this.$t(`user`),
            value: row?.user_name || ''
          },
          {
            key: this.$t(`oneterm.asset`),
            value: row?.asset_info || ''
          },
          {
            key: this.$t(`oneterm.gateway`),
            value: row?.gateway_info || ''
          },
          {
            key: this.$t(`oneterm.account`),
            value: row?.account_info || ''
          },
          {
            key: this.$t(`oneterm.protocol`),
            value: row?.protocol || ''
          },
          {
            key: this.$t(`oneterm.sessionTable.clientIp`),
            value: row?.client_ip || ''
          }
        ]

        const message = dataList.map((item) => {
          return `\x1b[38;2;138;226;52m${item.key}\x1b[38;2;110;172;218m: ${item.value}`
        }).join('; ')

        const data = [
          ``,
          `${message}\x1b[0m`,
          ``
        ]

        localStorage.setItem(initMessageStorageKey, JSON.stringify({
          timestamp: new Date().getTime(),
          data
        }))

        window.open(`/oneterm/terminal?session_id=${row.session_id}&is_monitor=true`, '_blank')
      }
    },
    disconnect(row) {
      this.loading = true
      closeConnect(row.session_id)
        .then((res) => {
          this.$message.success(this.$t('oneterm.sessionTable.disconnectSuccess'))
          this.updateTableData()
        })
        .finally(() => {
          this.loading = false
        })
    },
    exportTable() {
      const opsTable = this.$refs.opsTable.getVxetableRef()
      const records = [...opsTable.getCheckboxRecords(), ...opsTable.getCheckboxReserveRecords()]
      this.$refs.opsTable.getVxetableRef().exportData({
        data: records,
        columnFilterMethod({ column }) {
          return !['checkbox'].includes(column.type)
        },
      })
    },
  },
}
</script>

<style lang="less" scoped>
@import '../../style/index.less';
</style>
