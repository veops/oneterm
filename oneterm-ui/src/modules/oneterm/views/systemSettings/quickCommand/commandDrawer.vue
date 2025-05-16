<template>
  <CustomDrawer
    width="500px"
    :title="$t('oneterm.quickCommand.name')"
    :visible="visible"
    :zIndex="1003"
    @close="handleClose"
  >
    <a-input-search
      v-model="filterName"
      allow-clear
      class="ops-input ops-input-radius command-search"
      :placeholder="$t('placeholderSearch')"
      @search="updateTableData()"
    />

    <ops-table
      size="small"
      ref="opsTable"
      stripe
      class="ops-stripe-table"
      :data="tableData"
      show-overflow
      show-header-overflow
      :height="tableHeight"
      resizable
    >
      <vxe-column :title="$t('name')" field="name"> </vxe-column>
      <vxe-column :title="$t('description')" field="description"> </vxe-column>
      <vxe-column :title="$t('oneterm.command')" field="command"> </vxe-column>
      <vxe-column :title="$t('operation')" width="100">
        <template #default="{row}">
          <a @click="writeCommand(row)">
            {{ $t('oneterm.quickCommand.use') }}
          </a>
        </template>
      </vxe-column>
    </ops-table>

    <div class="command-pagination">
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
        :page-size-options="pageSizeOptions"
        @change="pageOrSizeChange"
        @showSizeChange="pageOrSizeChange"
      />
    </div>
  </CustomDrawer>
</template>

<script>
import { mapState } from 'vuex'
import { getQuickCommand } from '@/modules/oneterm/api/quickCommand.js'

export default {
  name: 'CommandDrawer',
  data() {
    return {
      visible: false,
      filterName: '',

      pageSizeOptions: ['20', '50', '100', '200'],
      tableData: [],
      currentPage: 1,
      pageSize: 20,
      totalResult: 0,
    }
  },
  computed: {
    ...mapState({
      windowHeight: (state) => state.windowHeight,
    }),
    tableHeight() {
      return this.windowHeight - 240
    }
  },
  methods: {
    open() {
      this.visible = true
      this.updateTableData()
    },

    updateTableData() {
      this.loading = true
      getQuickCommand({
        page_index: this.currentPage,
        page_size: this.pageSize,
        search: this.filterName,
      })
        .then((res) => {
          this.tableData = res?.data?.list || []
          this.totalResult = res?.data?.count ?? 0
        })
        .finally(() => {
          this.loading = false
        })
    },

    handleClose() {
      this.tableData = []
      this.currentPage = 1
      this.pageSize = 20
      this.totalResult = 0
      this.filterName = ''
      this.visible = false
    },

    pageOrSizeChange(currentPage, pageSize) {
      this.currentPage = currentPage
      this.pageSize = pageSize
      this.updateTableData()
    },

    writeCommand(data) {
      if (data?.command) {
        this.$emit('write', data.command)
      }
    }
  }
}
</script>

<stype lang="less" scoped>
.command-search {
  width: 250px;
  margin-bottom: 20px;
}

.command-pagination {
  text-align: right;
  margin-top: 8px;
}
</stype>
