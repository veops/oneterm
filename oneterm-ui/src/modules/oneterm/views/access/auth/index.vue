<template>
  <div class="access-auth">
    <a-spin :tip="loadTip" :spinning="loading">
      <div class="access-auth-header">
        <a-space>
          <a-input-search
            v-model="searchValue"
            allow-clear
            :placeholder="$t('placeholderSearch')"
            :style="{ width: '250px' }"
            class="ops-input ops-input-radius"
            @search="updateTableData()"
          />
          <div class="ops-list-batch-action" v-show="!!selectedRowKeys.length">
            <span @click="batchDelete">{{ $t('delete') }}</span>
            <span @click="batchChangeEnabled(true)">{{ $t('oneterm.enabled') }}</span>
            <span @click="batchChangeEnabled(false)">{{ $t('oneterm.disabled') }}</span>
            <span>{{ $t('selectRows', { rows: selectedRowKeys.length }) }}</span>
          </div>
        </a-space>
        <a-space>
          <a-button type="primary" @click="openAuthDrawer(null)">{{ $t(`create`) }}</a-button>
          <a-button @click="updateTableData()">{{ $t(`refresh`) }}</a-button>
        </a-space>
      </div>
      <ops-table
        size="small"
        ref="opsTable"
        class="ops-stripe-table"
        stripe
        show-overflow
        show-header-overflow
        resizable
        :data="tableData"
        :checkbox-config="{ reserve: true, highlight: true, range: true }"
        :row-config="{ keyField: 'id', height: '80px' }"
        :column-config="{ width: 200 }"
        :height="tableHeight"
        @checkbox-change="onSelectChange"
        @checkbox-all="onSelectChange"
        @checkbox-range-end="onSelectRangeEnd"
      >
        <vxe-column type="checkbox" width="60px"></vxe-column>
        <vxe-column :title="$t('name')" field="name"></vxe-column>
        <vxe-column :title="$t('description')" field="description" width="auto" min-width="200"></vxe-column>
        <vxe-column :title="$t('oneterm.isEnable')" field="enabled" width="100">
          <template #default="{row}">
            <EnabledStatus
              :status="Boolean(row.enabled)"
              @change="changeIsEnabled(row)"
            />
          </template>
        </vxe-column>
        <vxe-column :title="$t('created_at')" field="created_at">
          <template #default="{row}">
            {{ row.createdTimeText }}
          </template>
        </vxe-column>
        <vxe-column :title="$t('oneterm.auth.targetSelect')" field="targetSelect">
          <template #default="{row}">
            <a-tooltip
              v-for="(item, index) in row.targetSelect"
              :key="index"
              :title="item"
              placement="topLeft"
            >
              <div class="access-auth-target-select">
                {{ item }}
              </div>
            </a-tooltip>
          </template>
        </vxe-column>
        <vxe-column :title="$t('oneterm.accessControl.permissionConfig')" field="permissions" width="170">
          <template #default="{row}">
            <div class="access-auth-permisson">
              <span
                v-for="(item) in permissionConfigKeys"
                :key="item"
              >
                {{ $t(PERMISSION_TYPE_NAME[item]) }}
                <a-icon
                  v-if="row.permissions && row.permissions[item]"
                  type="check-square"
                  style="color: #00b42a"
                />
                <a-icon
                  v-else
                  type="close-square"
                  style="color: #fd4c6a"
                />
              </span>
            </div>
          </template>
        </vxe-column>
        <vxe-column :title="$t('oneterm.auth.validTime')" field="validTime">
          <template #default="{row}">
            <div
              v-for="(item) in row.validTime"
              :key="item"
            >
              {{ item }}
            </div>
          </template>
        </vxe-column>
        <vxe-column :title="$t('operation')" width="100" fixed="right">
          <template #default="{row}">
            <a-space>
              <a @click="openAuthDrawer(row)"><ops-icon type="icon-xianxing-edit"/></a>
              <a @click="copyAuth(row)"><ops-icon type="veops-copy"/></a>
              <a-popconfirm :title="$t('confirmDelete')" @confirm="deleteAuth(row)">
                <a style="color:red"><ops-icon type="icon-xianxing-delete"/></a>
              </a-popconfirm>
            </a-space>
          </template>
        </vxe-column>
      </ops-table>
      <div class="access-auth-pagination">
        <a-pagination
          size="small"
          show-size-changer
          :current="currentPage"
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
          :default-current="1"
          @change="pageOrSizeChange"
          @showSizeChange="pageOrSizeChange"
        />
      </div>
    </a-spin>
    <AuthDrawer ref="authDrawerRef" @submit="updateTableData()" />
  </div>
</template>

<script>
import _ from 'lodash'
import moment from 'moment'
import { mapState } from 'vuex'
import { getAuthList, deleteAuthById, putAuthById } from '@/modules/oneterm/api/authorizationV2.js'
import { getAllDepAndEmployee } from '@/api/company'
import { TARGET_SELECT_TYPE } from './constants.js'
import { PERMISSION_TYPE_NAME, PERMISSION_TYPE } from '@/modules/oneterm/views/systemSettings/accessControl/constants.js'

import AuthDrawer from './authDrawer/index.vue'
import EnabledStatus from '@/components/EnabledStatus/index.vue'

export default {
  name: 'AccessAuth',
  components: {
    AuthDrawer,
    EnabledStatus
  },
  provide() {
    return {
      provide_allTreeDepAndEmp: () => {
        return this.allTreeDepAndEmp
      },
    }
  },
  data() {
    return {
      PERMISSION_TYPE_NAME,
      searchValue: '',

      tableData: [],
      currentPage: 1,
      pageSize: 20,
      totalResult: 0,
      selectedRowKeys: [],

      loading: false,
      loadTip: '',
      permissionConfigKeys: Object.values(PERMISSION_TYPE),
      authRawKeys: [
        'access_control',
        'account_selector',
        'node_selector',
        'asset_selector',
        'description',
        'enabled',
        'name',
        'permissions',
        'rids',
        'valid_from',
        'valid_to'
      ],
      allTreeDepAndEmp: [],
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
    }),
    tableHeight() {
      return this.windowHeight - 204
    }
  },
  mounted() {
    this.updateTableData()
    getAllDepAndEmployee({ block: 0 }).then((res) => {
      this.allTreeDepAndEmp = res
    })
  },
  methods: {
    updateTableData() {
      this.loading = true

      getAuthList({
        page_index: this.currentPage,
        page_size: this.pageSize,
        search: this.searchValue
      })
        .then((res) => {
          const tableData = res?.data?.list || []
          tableData.forEach((row) => {
            row.createdTimeText = moment(row.created_at).format('YYYY-MM-DD HH:mm:ss')

            if (row.valid_from && row.valid_to) {
              row.validTime = [
                `${this.$t('oneterm.auth.start')}: ${row.valid_from}`,
                `${this.$t('oneterm.auth.end')}: ${row.valid_to}`
              ]
            } else {
              row.validTime = [
                this.$t('oneterm.auth.permanentValidity')
              ]
            }
            row.targetSelect = [
              `${this.$t('oneterm.node')}: ${this.handleTargetSelectText(row.node_selector)}`,
              `${this.$t('oneterm.asset')}: ${this.handleTargetSelectText(row.asset_selector)}`,
              `${this.$t('oneterm.account')}: ${this.handleTargetSelectText(row.account_selector)}`
            ]
          })
          this.tableData = tableData
          this.totalResult = res?.data?.count ?? 0
        })
        .finally(() => {
          this.loading = false
        })
    },
    handleTargetSelectText(data) {
      const values = data?.values || []

      switch (data.type) {
        case TARGET_SELECT_TYPE.ALL:
          return this.$t('oneterm.auth.all')
        case TARGET_SELECT_TYPE.ID:
          return this.$t('oneterm.auth.select', { count: values.length })
        case TARGET_SELECT_TYPE.REGEX:
          return `${this.$t('oneterm.auth.regex')} (${values.join(', ')})`
        case TARGET_SELECT_TYPE.TAG:
          return `${this.$t('oneterm.auth.tag')} (${values.join(', ')})`
        default:
          return ''
      }
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
      this.currentPage = currentPage
      this.pageSize = pageSize
      this.updateTableData()
    },
    openAuthDrawer(data) {
      this.$refs.authDrawerRef.open(data)
    },
    deleteAuth(row) {
      this.loading = true
      deleteAuthById(row.id)
        .then((res) => {
          this.$message.success(this.$t('deleteSuccess'))
          this.updateTableData()
        })
        .finally(() => {
          this.loading = false
        })
    },
    copyAuth(row) {
      const data = _.omit(_.cloneDeep(row), 'id')
      data.name += '-copy'
      this.$refs.authDrawerRef.open(data)
    },
    async batchDelete() {
      this.$confirm({
        title: this.$t('warning'),
        content: this.$t('confirmDelete'),
        onOk: async () => {
          let successNum = 0
          let errorNum = 0
          this.loading = true
          this.loadTip = `${this.$t('deleting')}...`
          for (let i = 0; i < this.selectedRowKeys.length; i++) {
            await deleteAuthById(this.selectedRowKeys[i])
              .then(() => {
                successNum += 1
              })
              .catch(() => {
                errorNum += 1
              })
              .finally(() => {
                this.loadTip = this.$t('deletingTip', { total: this.selectedRowKeys.length, successNum, errorNum })
              })
          }
          this.afterBatch()
        },
      })
    },
    batchChangeEnabled(enabled) {
      this.$confirm({
        title: this.$t('warning'),
        content: this.$t('oneterm.confirmEnable'),
        onOk: async () => {
          const opsTable = this.$refs.opsTable.getVxetableRef()
          const records = [...opsTable.getCheckboxRecords(), ...opsTable.getCheckboxReserveRecords()]

          let successNum = 0
          let errorNum = 0
          this.loading = true
          this.loadTip = `${this.$t('oneterm.switching')}...`

          for (let i = 0; i < records.length; i++) {
            const params = _.pick(_.cloneDeep(records[i]), this.authRawKeys)
            params.enabled = enabled

            await putAuthById(records[i].id, params)
              .then(() => {
                successNum += 1
              })
              .catch(() => {
                errorNum += 1
              })
              .finally(() => {
                this.loadTip = this.$t('oneterm.switchingTip', { total: records.length, successNum, errorNum })
              })
          }
          this.afterBatch()
        },
      })
    },
    afterBatch() {
      this.loading = false
      this.loadTip = ''
      this.selectedRowKeys = []
      this.$refs.opsTable.getVxetableRef().clearCheckboxRow()
      this.$refs.opsTable.getVxetableRef().clearCheckboxReserve()
      this.$nextTick(() => {
        this.updateTableData()
      })
    },
    changeIsEnabled(row) {
      const params = _.pick(_.cloneDeep(row), this.authRawKeys)
      params.enabled = !params.enabled
      putAuthById(row.id, params).then(() => {
        this.$message.success(this.$t('editSuccess'))
        this.updateTableData()
      })
    }
  },
}
</script>

<style lang="less" scoped>
.access-auth {
  background-color: #fff;
  height: 100%;
  border-radius: 6px;
  padding: 18px;

  /deep/ .vxe-body--row {
    height: 80px;
  }

  /deep/ .vxe-cell {
    max-height: max-content !important;
  }

  &-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  &-target-select {
    overflow: hidden;
    text-overflow: ellipsis;
    text-wrap: nowrap;
  }

  &-permisson {
    display: flex;
    flex-wrap: wrap;
    column-gap: 8px;

    & > span {
      width: 45%;
    }
  }

  &-pagination {
    text-align: right;
    margin-top: 8px;
  }
}
</style>
