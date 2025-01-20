<template>
  <div class="oneterm-layout">
    <div class="oneterm-header">{{ $t('oneterm.menu.operationLog') }}</div>
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
              @search="getOperationLog()"
            />
            <ChartTime
              class="oneterm-charttime"
              ref="chartTime"
              :list="chartTimeTagsList"
              localStorageKey="oneterm-operation-log"
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
            <a-button @click="getOperationLog()">{{ $t(`refresh`) }}</a-button>
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
          <vxe-column type="checkbox" width="60px" field="checkbox" ></vxe-column>
          <vxe-column type="expand" width="60px" field="expand">
            <template #content="{ row }">
              <div style="padding: 30px;">
                <a-row>
                  <a-col
                    :span="4"
                  ><span
                  ><strong>{{ $t('oneterm.log.param') }}</strong>
                  </span></a-col
                  >
                  <a-col
                    :span="10"
                  ><span
                  ><strong>{{ $t('oneterm.log.before') }}</strong>
                  </span></a-col
                  >
                  <a-col
                    :span="10"
                  ><span
                  ><strong>{{ $t('oneterm.log.after') }}</strong>
                  </span></a-col
                  >
                </a-row>
                <a-row
                  :gutter="[0, 6]"
                  v-for="item in compareObjects(row.new, row.old, row.action_type)"
                  :key="item.key"
                >
                  <a-col :span="4">
                    <a-tag color="blue">
                      <span>{{ item.key }}</span>
                    </a-tag>
                  </a-col>
                  <a-col :span="10" :style="{ whiteSpace: 'normal', wordBreak: 'break-word' }">
                    {{ item.old }}
                  </a-col>
                  <a-col :span="10" :style="{ whiteSpace: 'normal', wordBreak: 'break-word' }">
                    {{ item.new }}
                  </a-col>
                </a-row>
              </div>
            </template>
          </vxe-column>
          <vxe-column :title="$t('oneterm.log.time')" field="created_at" >
            <template #default="{row}">
              {{ moment(row.created_at).format('YYYY-MM-DD HH:mm:ss') }}
            </template>
          </vxe-column>
          <vxe-column :title="$t('user')" field="creator_id" cell-type="string">
            <template #default="{row}">
              {{ findNickname(row.creator_id) }}
            </template>
          </vxe-column>
          <vxe-column :title="$t(`operation`)" field="action_type" cell-type="string">
            <template #default="{row}">
              <a-tag color="green" v-if="row.action_type === 1">
                {{ $t('new') }}
              </a-tag>
              <a-tag color="red" v-if="row.action_type === 2">
                {{ $t('delete') }}
              </a-tag>
              <a-tag color="orange" v-if="row.action_type === 3">
                {{ $t('update') }}
              </a-tag>
            </template>
          </vxe-column>
          <vxe-column :title="$t('oneterm.log.type')" field="type">
            <template #default="{row}">
              {{ resourceMap[row.type] }}
            </template>
          </vxe-column>
          <!-- <vxe-column width="60px">
            <template #header>
                <ops-icon type="ops-itsm-ticketsetting"></ops-icon>
            </template>
          </vxe-column> -->
        </ops-table>
        <div class="oneterm-layout-pagination">
          <a-pagination
            size="small"
            show-size-changer
            v-model="tablePage.currentPage"
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
import _ from 'lodash'
import ChartTime from '@/components/chartTime'
import { getOperationLogList, getResourceType } from '../../../api/operationLog'
import { getAccountList } from '../../../api/account'
import { getCommandList } from '../../../api/command'
import { chartTimeTagsList } from '../constants.js'
import { setLocalStorage } from '../../../utils'

export default {
  name: 'OperationLog',
  components: {
    ChartTime,
  },
  data() {
    return {
      chartTimeTagsList,
      searchValue: '',
      tableData: [],
      selectedRowKeys: [],
      loading: false,
      tablePage: {
        currentPage: 1,
        pageSize: 50,
        totalResult: 0,
      },
      resourceMap: {},
      selectedData: [],
      diffArr: [],
      textareaValue: '',
      accountList: [],
      commandList: [],
    }
  },
  computed: {
    windowHeight() {
      return this.$store.state.windowHeight
    },
    tableHeight() {
      return this.windowHeight - 258
    },
    allEmployees() {
      return this.$store.state.user.allEmployees
    },
  },
  mounted() {
    this.getResourceTypeMap()
    this.getAccount()
    this.getCommand()
  },
  methods: {
    moment,
    compareObjects(n, o, type) {
      const res = []
      function getDiff(n, o, type) {
        if (type === 1) {
          for (const key in n) {
            if (Object.prototype.toString.call(n[key]) === '[object Object]') {
              getDiff(n[key], null, type)
            } else {
              res.push({
                key: key,
                new: n[key],
                action_type: type,
              })
            }
          }
        } else if (type === 2) {
          for (const key in o) {
            if (Object.prototype.toString.call(o[key]) === '[object Object]') {
              getDiff(null, o[key], type)
            } else {
              res.push({
                key: key,
                old: o[key],
                action_type: type,
              })
            }
          }
        } else {
          for (const key in n) {
            if (!_.isEqual(n[key], o[key])) {
              if (
                Object.prototype.toString.call(n[key]) === '[object Object]' &&
                Object.prototype.toString.call(o[key]) === '[object Object]'
              ) {
                getDiff(n[key], o[key], type)
              } else {
                res.push({
                  key: key,
                  old: o[key],
                  new: n[key],
                  action_type: type,
                })
              }
            }
          }
        }
      }
      getDiff(n, o, type)
      console.log(res, 'resresres')
      res.forEach((item) => {
        if (item.key === 'user_rids') {
          if (item.new) {
            item.new = item.new.map((id) => this.findNickname(id, 'acl_rid')).filter((item) => item !== undefined)
          }
          if (item.old) {
            item.old = item.old.map((id) => this.findNickname(id, 'acl_rid')).filter((item) => item !== undefined)
          }
        }
        if (item.key === 'account_ids') {
          if (item.new) {
            item.new = item.new.map((id) => this.findAccountname(id)).filter((item) => item !== undefined)
          }
          if (item.old) {
            item.old = item.old.map((id) => this.findAccountname(id)).filter((item) => item !== undefined)
          }
        }
        if (item.key === 'start' || item.key === 'end') {
          if (item.new) {
            item.new = moment(item.new).format('YYYY-MM-DD HH:mm:ss')
          }
          if (item.old) {
            item.old = moment(item.old).format('YYYY-MM-DD HH:mm:ss')
          }
        }
        if (item.key === 'account_type') {
          if (item.new) {
            if (item.new === 1) {
              item.new = this.$t('oneterm.password')
            } else {
              item.new = this.$t('oneterm.secretkey')
            }
          }
          if (item.old) {
            if (item.old === 1) {
              item.old = this.$t('oneterm.password')
            } else {
              item.old = this.$t('oneterm.secretkey')
            }
          }
        }
        if (item.key === 'cmds') {
          if (item.new) {
            item.new = this.commandList
              .map((command) => {
                if (_.isEqual(item.new, command.cmds)) {
                  return command.name
                }
              })
              .filter((item) => item !== undefined)
          }
          if (item.old) {
            item.old = this.commandList
              .map((command) => {
                if (_.isEqual(item.old, command.cmds)) {
                  return command.name
                }
              })
              .filter((item) => item !== undefined)
          }
        }
      })
      const excludeList = ['creator_id', 'created_at', 'updated_at', 'updater_id', 'resource_id', 'id']
      if (type === 1) {
        return res.filter((item) => {
          return item.new !== null && item.new !== '' && !excludeList.includes(item.key)
        })
      }
      if (type === 2) {
        return res.filter((item) => {
          return item.old !== null && item.old !== '' && !excludeList.includes(item.key)
        })
      }
      if (type === 3) {
        return res.filter((item) => {
          return !excludeList.includes(item.key)
        })
      }
    },
    findNickname(id, findKey = 'acl_uid') {
      const _find = this.allEmployees.find((item) => item[`${findKey}`] === id)
      return _find?.nickname
    },
    findAccountname(id) {
      const _find = this.accountList.find((item) => item.id === id)
      return _find?.name
    },
    getCommand() {
      getCommandList({
        page_index: 1,
      }).then((res) => {
        this.commandList = res?.data?.list || []
      })
    },
    getAccount() {
      getAccountList({
        page_index: 1,
      }).then((res) => {
        this.accountList = res?.data?.list || []
      })
    },
    getResourceTypeMap() {
      getResourceType().then((res) => {
        this.resourceMap = res.data
      })
    },
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
    getOperationLog(currentPage = 1, pageSize = this.tablePage.pageSize) {
      this.loading = true
      getOperationLogList({
        ...this.startAndEnd(),
        search: this.searchValue,
        page_index: currentPage,
        page_size: pageSize,
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
    chartTimeChange({ from_ts, to_ts, isFixedTime, intervalTime, range_date_remeber, range_date, timeType }) {
      if (isFixedTime) {
        setLocalStorage('oneterm-operation-log', {
          range_date_detail: range_date,
          isFixedTime,
          range_date: range_date_remeber,
        })
        this.showTime = { from_ts, to_ts, isFixedTime }
      } else {
        const { number, valueFormat, type } = range_date_remeber
        setLocalStorage('oneterm-operation-log', { isFixedTime, range_date: range_date_remeber })
        this.showTime = { isFixedTime, type, number, valueFormat }
      }
      this.getOperationLog(1)
    },
    onSelectChange() {
      const opsTable = this.$refs.opsTable.getVxetableRef()
      const records = [...opsTable.getCheckboxRecords(), ...opsTable.getCheckboxReserveRecords()]
      this.selectedRowKeys = records.map((i) => i.id)
    },
    onSelectRangeEnd({ records }) {
      this.selectedRowKeys = records.map((i) => i.id)
    },
    toExport() {
      const actionTypeMap = {
        1: this.$t('new'),
        2: this.$t('delete'),
        3: this.$t('update'),
      }

      const data = this.$refs.opsTable
        .getVxetableRef()
        .getCheckboxRecords()
        .map((item) => {
          return {
            ...item,
            created_at: moment(item.created_at).format('YYYY-MM-DD HH:mm:ss'),
            type: this.resourceMap[item.type],
            creator_id: this.findNickname(item.creator_id),
            action_type: actionTypeMap[item.action_type]
          }
        })

      this.$refs.opsTable.getVxetableRef().exportData({
        data,
        filename: this.$t('oneterm.menu.operationLog'),
        sheetName: 'Sheet1',
        type: 'xlsx',
        types: ['xlsx', 'csv', 'html', 'xml', 'txt'],
        isFooter: false,
        columnFilterMethod: function(column) {
          return ['created_at', 'creator_id', 'action_type', 'type'].includes(column.column.field)
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
    pageOrSizeChange(currentPage, pageSize) {
      this.getOperationLog(currentPage, pageSize)
    },
  },
}
</script>

<style lang="less">
@import '../../../style/index.less';
</style>
