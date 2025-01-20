<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ $t('oneterm.menu.loginLog') }}</div>
    <a-spin :spinning="loading">
      <div class="oneterm-layout-container">
        <div class="oneterm-layout-container-header">
          <a-space>
            <a-input-search
              allow-clear
              v-model="searchValue"
              :style="{ width: '250px' }"
              class="ops-input ops-input-radius"
              :placeholder="$t('placeholderSearch')"
              @search="getLoginlog"
            />
            <ChartTime
              class="oneterm-charttime"
              ref="chartTime"
              :list="chartTimeTagsList"
              localStorageKey="oneterm-login-log"
              :isShowInternalTime="false"
              :default_range_date_remeber="{
                number: 1,
                valueFormat: 'day',
              }"
              @chartTimeChange="chartTimeChange"
            >
              <a-icon type="calendar" slot="displayTimeIcon" class="primary-color" />
            </ChartTime>
            <div class="ops-list-batch-action" v-show="!!selectedRowKeys.length">
              <span @click="handleExport">{{ $t('export') }}</span>
              <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
            </div>
          </a-space>
          <a-space>
            <a-button @click="refresh">{{ $t(`refresh`) }}</a-button>
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
        >
          <vxe-column type="checkbox" width="60px"></vxe-column>
          <vxe-column :title="$t('user')" field="username"> </vxe-column>
          <vxe-column :title="$t('type')" field="channel" width="100px"> </vxe-column>
          <vxe-column :title="$t('oneterm.assetList.ip')" field="ip"> </vxe-column>
          <vxe-column :title="$t('browser')" field="browser"></vxe-column>
          <vxe-column :title="$t('description')" field="description"> </vxe-column>
          <vxe-column :title="$t('status')" align="center" width="110">
            <template #default="{ row }">
              <span v-if="row.is_ok" class="oneterm-table-right">{{ $t('success') }}</span>
              <span v-else class="oneterm-table-right oneterm-table-error">{{ $t('fail') }}</span>
            </template>
          </vxe-column>
          <vxe-column :title="$t('login_at')" field="login_at"> </vxe-column>
          <vxe-column :title="$t('logout_at')" field="logout_at"> </vxe-column>
        </ops-table>
        <Pager
          ref="pager"
          :current-page.sync="tablePage.page"
          :page-size.sync="tablePage.page_size"
          :page-sizes="[50, 100, 200]"
          :total="tableDataLength"
          :isLoading="loading"
          @change="onChange"
          @showSizeChange="onShowSizeChange"
          :style="{ marginTop: '10px' }"
        ></Pager>
      </div>
    </a-spin>
  </div>
</template>

<script>
import moment from 'moment'
import ChartTime from '@/components/chartTime'
import Pager from '@/components/Pager'
import { getLoginLogList } from '../../../api/loginLog'
import { chartTimeTagsList } from '../constants.js'
import { setLocalStorage } from '../../../utils'

export default {
  name: 'LogLogin',
  components: {
    ChartTime,
    Pager,
  },
  data() {
    return {
      chartTimeTagsList,
      loading: true,
      tableData: [],
      selectedRowKeys: [],
      tablePage: {
        page: 1,
        page_size: 50,
      },
      searchValue: '',
      showTime: {},
    }
  },
  computed: {
    windowHeight() {
      return this.$store.state.windowHeight
    },
    tableHeight() {
      return this.windowHeight - 258
    },
    tableDataLength() {
      return this.tableData.length
    },
  },
  methods: {
    moment,
    startAndEnd() {
      const params = {}
      if (this.showTime) {
        if (this.showTime.isFixedTime) {
          const { from_ts, to_ts } = this.showTime
          params['start'] = moment(from_ts * 1000).format('YYYY-MM-DD HH:mm:ss')
          params['end'] = moment(to_ts * 1000).format('YYYY-MM-DD HH:mm:ss')
        } else {
          const { number, valueFormat, type } = this.showTime
          if (type === 'Today') {
            params['start'] = moment()
              .startOf('day')
              .format('YYYY-MM-DD HH:mm:ss')
          } else if (type === 'This Month') {
            params['start'] = moment()
              .startOf('month')
              .format('YYYY-MM-DD HH:mm:ss')
          } else if (type === 'all') {
            params['start'] = '2023-01-01'
          } else {
            params['start'] = moment()
              .subtract(number, valueFormat)
              .format('YYYY-MM-DD HH:mm:ss')
          }
          params['end'] = moment().format('YYYY-MM-DD HH:mm:ss')
        }
      }
      return params
    },
    getLoginlog() {
      this.loading = true
      getLoginLogList({ ...this.tablePage, ...this.startAndEnd(), q: this.searchValue })
        .then((res) => {
          this.tableData = res?.data || []
        })
        .finally(() => {
          this.loading = false
        })
    },
    chartTimeChange({ from_ts, to_ts, isFixedTime, intervalTime, range_date_remeber, range_date, timeType }) {
      if (isFixedTime) {
        setLocalStorage('oneterm-login-log', {
          range_date_detail: range_date,
          isFixedTime,
          range_date: range_date_remeber,
        })
        this.showTime = { from_ts, to_ts, isFixedTime }
      } else {
        const { number, valueFormat, type } = range_date_remeber
        setLocalStorage('oneterm-login-log', { isFixedTime, range_date: range_date_remeber })
        this.showTime = { isFixedTime, type, number, valueFormat }
      }
      this.refresh()
    },
    onSelectChange() {
      const opsTable = this.$refs.opsTable.getVxetableRef()
      const records = [...opsTable.getCheckboxRecords(), ...opsTable.getCheckboxReserveRecords()]
      this.selectedRowKeys = records.map((i) => i.id)
    },
    onSelectRangeEnd({ records }) {
      this.selectedRowKeys = records.map((i) => i.id)
    },
    refresh() {
      this.tablePage.page = 1
      this.getLoginlog()
    },
    handleExport() {
      this.$refs.opsTable.getVxetableRef().exportData({
        data: this.$refs.opsTable
          .getVxetableRef()
          .getCheckboxRecords()
          .map((item) => {
            return {
              ...item,
            }
          }),
        filename: this.$t('oneterm.menu.loginLog'),
        sheetName: 'Sheet1',
        type: 'xlsx',
        types: ['xlsx', 'csv', 'html', 'xml', 'txt'],
        useStyle: true,
        isFooter: false,
        columnFilterMethod: function(column, $columnIndex) {
          return !(column.$columnIndex === 0)
        },
      })
      this.$refs.opsTable.getVxetableRef().clearCheckboxRow()
      this.$refs.opsTable.getVxetableRef().clearCheckboxReserve()
      this.$refs.opsTable.getVxetableRef().clearSort()
      this.selectedRowKeys = []
    },
    fillZero(str) {
      let realNum
      if (str < 10) {
        realNum = '0' + str
      } else {
        realNum = str
      }
      return realNum
    },
    onShowSizeChange(size) {
      this.tablePage.page_size = size
      this.tablePage.page = 1
      this.getLoginlog()
    },
    onChange(pageNum) {
      this.tablePage.page = pageNum
      this.getLoginlog()
    },
  },
}
</script>

<style lang="less">
@import '../../../style/index.less';
</style>
