<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ $t('oneterm.menu.fileLog') }}</div>
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
              @search="getFileLog()"
            />
            <ChartTime
              class="oneterm-charttime"
              ref="chartTime"
              :list="chartTimeTagsList"
              localStorageKey="oneterm-file-log"
              :isShowInternalTime="false"
              :default_range_date_remeber="{
                number: 1,
                valueFormat: 'month',
              }"
              @chartTimeChange="chartTimeChange"
            >
              <a-icon type="calendar" slot="displayTimeIcon" class="primary-color" />
            </ChartTime>
            <div class="ops-list-batch-action" v-show="!!selectedRowKeys.length">
              <span @click="toExport">{{ $t('export') }}</span>
              <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
            </div>
          </a-space>
          <a-space>
            <a-button @click="getFileLog()">{{ $t(`refresh`) }}</a-button>
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
          :checkbox-config="{ reserve: true, highlight: true, range: true }"
          :row-config="{ keyField: 'id' }"
          :height="tableHeight"
          resizable
          @checkbox-change="onSelectChange"
          @checkbox-all="onSelectChange"
          @checkbox-range-end="onSelectChange"
        >
          <vxe-column type="checkbox" width="60px" field="checkbox" ></vxe-column>
          <vxe-column :title="$t('oneterm.log.fileName')" field="filename" ></vxe-column>
          <vxe-column :title="$t('oneterm.log.filePath')" field="dir" ></vxe-column>
          <vxe-column :title="$t('oneterm.log.account')" field="user_name" ></vxe-column>
          <vxe-column :title="$t(`operation`)" field="action" cell-type="string">
            <template #default="{row}">
              <a-tag v-if="row.actionText" :color="row.actionColor">
                {{ $t(row.actionText) }}
              </a-tag>
            </template>
          </vxe-column>
          <vxe-column :title="$t('oneterm.log.time')" field="created_at" >
            <template #default="{row}">
              {{ row.createdTime }}
            </template>
          </vxe-column>
        </ops-table>
        <div class="oneterm-layout-pagination">
          <a-pagination
            size="small"
            show-size-changer
            v-model="currentPage"
            :total="totalResult"
            :show-total="
              (total, range) =>
                $t('pagination.total', {
                  range0: range[0],
                  range1: range[1],
                  total,
                })
            "
            :page-size="pageSize"
            :page-size-options="['20', '50', '100']"
            @change="pageOrSizeChange"
            @showSizeChange="pageOrSizeChange"
          />
        </div>
      </div>
    </a-spin>
  </div>
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import ChartTime from '@/components/chartTime'
import { getFileHistory } from '@/modules/oneterm/api/file.js'
import { chartTimeTagsList } from '../constants.js'
import { setLocalStorage } from '../../../utils'

export default {
  name: 'FileLog',
  components: {
    ChartTime,
  },
  data() {
    return {
      loading: false,
      chartTimeTagsList,
      searchValue: '',
      tableData: [],
      selectedRowKeys: [],
      currentPage: 1,
      pageSize: 50,
      totalResult: 0,
      actionTypeMap: {
        3: {
          text: 'oneterm.log.upload',
          color: 'blue'
        },
        4: {
          text: 'oneterm.log.download',
          color: 'green'
        }
      }
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight
    }),
    tableHeight() {
      return this.windowHeight - 258
    }
  },
  methods: {
    moment,
    startAndEnd() {
      const params = {}
      if (this.showTime) {
        if (this.showTime.isFixedTime) {
          const { from_ts, to_ts } = this.showTime
          params['start'] = moment(from_ts * 1000).format()
          params['end'] = moment(to_ts * 1000).format()
        } else {
          const { number, valueFormat, type } = this.showTime
          if (type === 'Today') {
            params['start'] = moment()
              .startOf('day')
              .format()
          } else if (type === 'This Month') {
            params['start'] = moment()
              .startOf('month')
              .format()
          } else if (type === 'all') {
            params['start'] = moment('2023-01-01').format()
          } else {
            params['start'] = moment()
              .subtract(number, valueFormat)
              .format()
          }
          params['end'] = moment().format()
        }
      }
      return params
    },
    getFileLog() {
      this.loading = true
      getFileHistory({
        ...this.startAndEnd(),
        search: this.searchValue,
        page_index: this.currentPage,
        page_size: this.pageSize,
      })
        .then((res) => {
          const tableData = res?.data?.list || []
          tableData.forEach((row) => {
            const createMoment = moment(row.created_at)
            row.createTimeStamp = createMoment.valueOf()
            row.createdTime = createMoment.format('YYYY-MM-DD HH:mm:ss')
            const actionType = this.actionTypeMap?.[row.action]
            if (actionType) {
              row.actionText = actionType.text
              row.actionColor = actionType.color
            }
          })
          this.tableData = tableData
          this.totalResult = res?.data?.count ?? 0
        })
        .finally(() => {
          this.loading = false
        })
    },
    chartTimeChange({ from_ts, to_ts, isFixedTime, intervalTime, range_date_remeber, range_date, timeType }) {
      if (isFixedTime) {
        setLocalStorage('oneterm-file-log', {
          range_date_detail: range_date,
          isFixedTime,
          range_date: range_date_remeber,
        })
        this.showTime = { from_ts, to_ts, isFixedTime }
      } else {
        const { number, valueFormat, type } = range_date_remeber
        setLocalStorage('oneterm-file-log', { isFixedTime, range_date: range_date_remeber })
        this.showTime = { isFixedTime, type, number, valueFormat }
      }
      this.currentPage = 1
      this.getFileLog()
    },
    onSelectChange({ records }) {
      this.selectedRowKeys = records.map((i) => i.id)
    },
    toExport() {
      const data = this.$refs.opsTable
        .getVxetableRef()
        .getCheckboxRecords()
        .sort((a, b) => b.createTimeStamp - a.createTimeStamp)
        .map((item) => {
          return {
            ...item,
            created_at: item.createdTime,
            action: this.$t(this.actionTypeMap?.[item.action]?.text || item.action)
          }
        })

      this.$refs.opsTable.getVxetableRef().exportData({
        data,
        filename: this.$t('oneterm.menu.fileLog'),
        sheetName: 'Sheet1',
        type: 'xlsx',
        types: ['xlsx', 'csv', 'html', 'xml', 'txt'],
        isFooter: false,
        columnFilterMethod: function(column) {
          return ['filename', 'dir', 'user_name', 'action', 'created_at'].includes(column.column.field)
        },
      })
      this.$refs.opsTable.getVxetableRef().clearCheckboxRow()
      this.$refs.opsTable.getVxetableRef().clearCheckboxReserve()
      this.selectedRowKeys = []
    },
    pageOrSizeChange(currentPage, pageSize) {
      this.currentPage = currentPage
      this.pageSize = pageSize
      this.getFileLog()
    },
  },
}
</script>

<style lang="less">
@import '../../../style/index.less';
</style>
