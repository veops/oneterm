<template>
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
    </div>
    <ops-table
      size="small"
      ref="opsTable"
      stripe
      class="ops-stripe-table"
      :data="tableData"
      show-overflow
      show-header-overflow
      :row-config="{ keyField: 'id' }"
      :height="tableHeight"
      resizable
    >
      <vxe-column :title="$t('oneterm.sessionTable.cmd')" field="cmd"> </vxe-column>
      <!-- <vxe-column :title="$t('oneterm.sessionTable.level')" field="level"> </vxe-column> -->
      <vxe-column :title="$t('oneterm.sessionTable.execute_at')" field="created_at" width="150px">
        <template #default="{row}">
          {{ moment(row.created_at).format('YYYY-MM-DD HH:mm:ss') }}
        </template>
      </vxe-column>
      <vxe-column :title="$t('oneterm.sessionTable.result')" field="result"> </vxe-column>
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
</template>

<script>
import moment from 'moment'
import { mapState } from 'vuex'
import { getSessionCmdList } from '../../api/session'
export default {
  name: 'SessionDetailTable',
  props: {
    session_id: {
      type: String,
      default: null,
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
      isDetail: false,
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
      getSessionCmdList(this.session_id, {
        page_index: currentPage,
        page_size: pageSize,
        search: this.filterName,
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
    pageOrSizeChange(currentPage, pageSize) {
      this.updateTableData(currentPage, pageSize)
    },
  },
}
</script>

<style lang="less" scoped>
@import '../../style/index.less';
</style>
